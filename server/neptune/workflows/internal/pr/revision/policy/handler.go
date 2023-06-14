package policy

import (
	"context"
	"github.com/google/go-github/v45/github"
	metricNames "github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	gh "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
	"time"
)

type githubActivities interface {
	GithubListPRReviews(ctx context.Context, request activities.ListPRReviewsRequest) (activities.ListPRReviewsResponse, error)
	GithubListTeamMembers(ctx context.Context, request activities.ListTeamMembersRequest) (activities.ListTeamMembersResponse, error)
}

type dismisser interface {
	Dismiss(ctx workflow.Context, revision revision.Revision, teams map[string][]string, currentApprovals []*github.PullRequestReview) ([]*github.PullRequestReview, error)
}

type policyFilter interface {
	Filter(teams map[string][]string, currentApprovals []*github.PullRequestReview, failedPolicies []activities.PolicySet) []activities.PolicySet
}

type notifier interface {
	Notify(ctx workflow.Context, workflowState *state.Workflow, roots map[string]revision.RootInfo)
}

type NewApprovalRequest struct {
	Revision string
}

type FailedPolicyHandler struct {
	ApprovalSignalChannel workflow.ReceiveChannel
	Dismisser             dismisser
	PolicyFilter          policyFilter
	GithubActivities      githubActivities
	PRNumber              int
	Org                   string
	Scope                 metrics.Scope
	Notifier              notifier
}

type Action int64

const (
	onApprovalSignal Action = iota
	onPollTick
	onSkip
)

func (f *FailedPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, roots map[string]revision.RootInfo, workflowResponses []terraform.Response) {
	scope := f.Scope.SubScopeWithTags(map[string]string{metricNames.RevisionTag: revision.Revision})
	failedPolicies := dedup(workflowResponses, scope)
	if len(failedPolicies) == 0 {
		return
	}

	var action Action
	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	s.AddReceive(f.ApprovalSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		action = onApprovalSignal
		if !more {
			return
		}
		var approvalRequest NewApprovalRequest
		c.Receive(ctx, &approvalRequest)
		// skip signal if it's not for the current revision
		if approvalRequest.Revision != revision.Revision {
			action = onSkip
		}
		scope.SubScopeWithTags(map[string]string{metricNames.SignalNameTag: "pr-approval"}).
			Counter(metricNames.SignalReceive).
			Inc(1)
	})
	onTimeout := func(f workflow.Future) {
		_ = f.Get(ctx, nil)
		action = onPollTick
		scope.SubScopeWithTags(map[string]string{metricNames.PollNameTag: "pr-approval"}).
			Counter(metricNames.PollTick).
			Inc(1)
	}
	cancelTimer, _ := s.AddTimeout(ctx, 5*time.Minute, onTimeout)

	for {
		if len(failedPolicies) == 0 {
			break
		}
		s.Select(ctx)
		switch action {
		case onSkip:
			continue
		case onPollTick:
			// TODO: evaluate a better polling rate for approvals, or remove all together
			cancelTimer()
			cancelTimer, _ = s.AddTimeout(ctx, 10*time.Minute, onTimeout)
		}
		// filter out approved policies and update check status onPollTick and onApprovalSignal actions
		failedPolicies = f.filterApprovedPolicies(ctx, revision, failedPolicies)
		f.updateCheckStatus(ctx, roots, workflowResponses, failedPolicies)
	}
}

func (f *FailedPolicyHandler) filterApprovedPolicies(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) []activities.PolicySet {
	// Fetch current approvals in activity
	var listPRReviewsResponse activities.ListPRReviewsResponse
	err := workflow.ExecuteActivity(ctx, f.GithubActivities.GithubListPRReviews, activities.ListPRReviewsRequest{
		Repo:     revision.Repo,
		PRNumber: f.PRNumber,
	}).Get(ctx, &listPRReviewsResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error(err.Error())
		return failedPolicies
	}

	// Fetch current policy team memberships
	teams := make(map[string][]string)
	for _, policy := range failedPolicies {
		members, err := f.fetchTeamMembers(ctx, revision.Repo, policy.Owner)
		if err != nil {
			workflow.GetLogger(ctx).Error(err.Error())
			return failedPolicies
		}
		teams[policy.Name] = members
	}

	// Dismiss stale approvals
	currentReviews := listPRReviewsResponse.Reviews
	currentReviews, err = f.Dismisser.Dismiss(ctx, revision, teams, currentReviews)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to dismiss stale reviews")
		return failedPolicies
	}

	// Filter out failed policies from policy approvals
	filteredPolicies := f.PolicyFilter.Filter(teams, currentReviews, failedPolicies)
	return filteredPolicies
}

func (f *FailedPolicyHandler) fetchTeamMembers(ctx workflow.Context, repo gh.Repo, slug string) ([]string, error) {
	var listTeamMembersResponse activities.ListTeamMembersResponse
	err := workflow.ExecuteActivity(ctx, f.GithubActivities.GithubListTeamMembers, activities.ListTeamMembersRequest{
		Repo:     repo,
		Org:      f.Org,
		TeamSlug: slug,
	}).Get(ctx, &listTeamMembersResponse)
	return listTeamMembersResponse.Members, err
}

func (f *FailedPolicyHandler) updateCheckStatus(ctx workflow.Context, roots map[string]revision.RootInfo, responses []terraform.Response, policies []activities.PolicySet) {
	for _, response := range responses {
		if containsFailingPolicy(response.ValidationResults, policies) {
			continue
		}
		workflowState := response.WorkflowState
		workflowState.Result.Status = state.CompleteWorkflowStatus
		workflowState.Result.Reason = state.BypassedFailedValidationReason
		f.Notifier.Notify(ctx, &workflowState, roots)
	}
}

func containsFailingPolicy(results []activities.ValidationResult, failingPolicies []activities.PolicySet) bool {
	for _, result := range results {
		for _, policy := range failingPolicies {
			if result.PolicySet.Name == policy.Name {
				return true
			}
		}
	}
	return false
}

func dedup(workflowResponses []terraform.Response, scope metrics.Scope) []activities.PolicySet {
	var failedPolicies []activities.PolicySet
	for _, response := range workflowResponses {
		for _, validationResult := range response.ValidationResults {
			switch validationResult.Status {
			case activities.Fail:
				failedPolicies = append(failedPolicies, validationResult.PolicySet)
				scope.SubScope(validationResult.PolicySet.Name).Counter(metricNames.ExecutionFailureMetric).Inc(1)
			case activities.Success:
				scope.SubScope(validationResult.PolicySet.Name).Counter(metricNames.ExecutionSuccessMetric).Inc(1)
			}
		}
	}

	uniqueFailedPolicies := make(map[string]activities.PolicySet)
	for _, failedPolicy := range failedPolicies {
		uniqueFailedPolicies[failedPolicy.Name] = failedPolicy
	}
	return toSlice(uniqueFailedPolicies)
}

func toSlice(policyMap map[string]activities.PolicySet) []activities.PolicySet {
	var policies []activities.PolicySet
	for _, policy := range policyMap {
		policies = append(policies, policy)
	}
	return policies
}
