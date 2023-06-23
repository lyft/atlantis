package policy

import (
	"context"
	"time"

	"github.com/google/go-github/v45/github"
	metricNames "github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	gh "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/workflow"
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

type NewReviewRequest struct {
	Revision string
}

type FailedPolicyHandler struct {
	ReviewSignalChannel workflow.ReceiveChannel
	Dismisser           dismisser
	PolicyFilter        policyFilter
	GithubActivities    githubActivities
	PRNumber            int
	Org                 string
	Scope               metrics.Scope
	Notifier            notifier
}

type Action int64

const (
	onApprovalSignal Action = iota
	onPollTick
	onSkip
)

// Handle processes the roots corresponding to each Terraform workflow response and determines if any policies are failing
// and need manual approvals from policy owners.
func (f *FailedPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, roots map[string]revision.RootInfo, terraformWorkflowResponses []terraform.Response) {
	scope := f.Scope.SubScopeWithTags(map[string]string{metricNames.RevisionTag: revision.Revision})
	remainingFailedPolicies := fetchAllFailingPolicies(terraformWorkflowResponses, scope)
	if len(remainingFailedPolicies) == 0 {
		return
	}
	// we don't want to notify any workflows that were successful before we started polling for approvals
	_, failingTerraformWorkflowResponses := partitionWorkflowsByFailure(terraformWorkflowResponses, remainingFailedPolicies)

	var action Action
	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	s.AddReceive(f.ReviewSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		action = onApprovalSignal
		if !more {
			return
		}
		var approvalRequest NewReviewRequest
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
		if len(remainingFailedPolicies) == 0 {
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
		// onPollTick and onApprovalSignal actions, filter out any newly approved policies
		failingPolicies := f.filterOutApprovedPolicies(ctx, revision, remainingFailedPolicies)
		// check if we've filtered out any newly approved policies
		if len(failingPolicies) < len(remainingFailedPolicies) {
			// notify any workflows that are now newly successful
			successfulWorkflows, failingWorkflows := partitionWorkflowsByFailure(failingTerraformWorkflowResponses, failingPolicies)
			f.updateCheckStatuses(ctx, roots, successfulWorkflows)
			failingTerraformWorkflowResponses = failingWorkflows
			remainingFailedPolicies = failingPolicies
		}
	}
}

func (f *FailedPolicyHandler) filterOutApprovedPolicies(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) []activities.PolicySet {
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

// partitionWorkflowsByFailure separates workflows into successful and failing workflows
// based on the provided failing policies
func partitionWorkflowsByFailure(workflows []terraform.Response, failingPolicies []activities.PolicySet) ([]terraform.Response, []terraform.Response) {
	var failingWorkflows []terraform.Response
	var successfulWorkflows []terraform.Response
	for _, workflow := range workflows {
		if containsAnyPolicy(workflow, failingPolicies) {
			failingWorkflows = append(failingWorkflows, workflow)
		} else {
			successfulWorkflows = append(successfulWorkflows, workflow)
		}
	}
	return successfulWorkflows, failingWorkflows
}

// updateCheckStatuses checks to see if each root's workflow has any remaining failing policies.
// If it doesn't then the workflow's check status reason is updated to be bypassed (i.e. passing)
func (f *FailedPolicyHandler) updateCheckStatuses(ctx workflow.Context, roots map[string]revision.RootInfo, successfulWorkflows []terraform.Response) {
	numUpdates := 0
	for _, successfulWorkflow := range successfulWorkflows {
		sw := successfulWorkflow
		workflow.Go(ctx, func(c workflow.Context) {
			defer func() {
				numUpdates++
			}()
			workflowState := sw.WorkflowState
			workflowState.Result.Status = state.CompleteWorkflowStatus
			workflowState.Result.Reason = state.BypassedFailedValidationReason
			f.Notifier.Notify(c, &workflowState, roots)
		})
	}
	err := workflow.Await(ctx, func() bool {
		return numUpdates == len(successfulWorkflows)
	})
	if err != nil {
		workflow.GetLogger(ctx).Error(err.Error())
	}
}

// containsAnyPolicy checks if any of the provided failing policies are present in the provided workflow
func containsAnyPolicy(workflowResponse terraform.Response, failingPolicies []activities.PolicySet) bool {
	for _, response := range workflowResponse.ValidationResults {
		for _, policy := range failingPolicies {
			if response.PolicySet.Name == policy.Name {
				return true
			}
		}
	}
	return false
}

func fetchAllFailingPolicies(workflowResponses []terraform.Response, scope metrics.Scope) []activities.PolicySet {
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
