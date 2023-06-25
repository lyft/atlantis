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
	onReviewSignal Action = iota
	onPollTick
	onSkip
	onShutdown
)

// Handle processes the roots corresponding to each Terraform workflow response and determines if any policies are failing
// and need manual approvals from policy owners.
func (f *FailedPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, roots map[string]revision.RootInfo, failingTerraformWorkflows []terraform.Response) {
	scope := f.Scope.SubScopeWithTags(map[string]string{metricNames.RevisionTag: revision.Revision})
	if len(failingTerraformWorkflows) == 0 {
		return
	}

	var action Action
	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	s.AddReceive(ctx.Done(), func(c workflow.ReceiveChannel, more bool) {
		action = onShutdown
		scope.Counter(metricNames.ContextCancel).Inc(1)
	})
	s.AddReceive(f.ReviewSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		action = onReviewSignal
		if !more {
			return
		}
		var reviewRequest NewReviewRequest
		c.Receive(ctx, &reviewRequest)
		// skip signal if it's not for the current revision
		if reviewRequest.Revision != revision.Revision {
			action = onSkip
		}
		scope.SubScopeWithTags(map[string]string{metricNames.SignalNameTag: "pr-review"}).
			Counter(metricNames.SignalReceive).
			Inc(1)
	})
	onTimeout := func(f workflow.Future) {
		_ = f.Get(ctx, nil)
		action = onPollTick
		scope.SubScopeWithTags(map[string]string{metricNames.PollNameTag: "pr-review"}).
			Counter(metricNames.PollTick).
			Inc(1)
	}
	cancelTimer, _ := s.AddTimeout(ctx, 10*time.Minute, onTimeout)

	for {
		if len(failingTerraformWorkflows) == 0 {
			break
		}
		s.Select(ctx)
		switch action {
		case onShutdown:
			return
		case onSkip:
			continue
		case onPollTick:
			// TODO: evaluate a better polling rate for approvals, or remove all together
			cancelTimer()
			cancelTimer, _ = s.AddTimeout(ctx, 10*time.Minute, onTimeout)
		}

		// onPollTick and onReviewSignal actions, filter out failing policies that have been approved and identify if
		// any previously failing terraform workflows are now successful
		remainingFailedPolicies := f.filterOutBypassedPolicies(ctx, revision, failingTerraformWorkflows)
		successfulTerraformWorkflows := partitionWorkflowsByResult(failingTerraformWorkflows, remainingFailedPolicies, true)
		failingTerraformWorkflows = partitionWorkflowsByResult(failingTerraformWorkflows, remainingFailedPolicies, false)
		// for newly successful workflows, update their corresponding check statuses to passing
		f.updateCheckStatuses(ctx, roots, successfulTerraformWorkflows)
	}
}

func (f *FailedPolicyHandler) filterOutBypassedPolicies(ctx workflow.Context, revision revision.Revision, failingTerraformWorkflowResponses []terraform.Response) []activities.PolicySet {
	// Process set of currently failing policies
	failedPolicies := fetchAllFailingPolicies(failingTerraformWorkflowResponses)

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
	// TODO gate dismissal with flag
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

func partitionWorkflowsByResult(workflows []terraform.Response, failingPolicies []activities.PolicySet, success bool) []terraform.Response {
	var partitionedWorkflows []terraform.Response
	for _, workflow := range workflows {
		if success && !containsAnyFailingPolicy(workflow, failingPolicies) {
			partitionedWorkflows = append(partitionedWorkflows, workflow)
		} else if !success && containsAnyFailingPolicy(workflow, failingPolicies) {
			partitionedWorkflows = append(partitionedWorkflows, workflow)
		}
	}
	return partitionedWorkflows
}

// updateCheckStatuses marks each successful TF workflow's corresponding check status as passing (i.e. each root)
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

// containsAnyFailingPolicy checks if any of the provided failing policies are present in the provided workflow
func containsAnyFailingPolicy(workflowResponse terraform.Response, failingPolicies []activities.PolicySet) bool {
	for _, response := range workflowResponse.ValidationResults {
		for _, policy := range failingPolicies {
			if response.PolicySet.Name == policy.Name {
				return true
			}
		}
	}
	return false
}

func fetchAllFailingPolicies(workflowResponses []terraform.Response) []activities.PolicySet {
	var failedPolicies []activities.PolicySet
	for _, response := range workflowResponses {
		for _, validationResult := range response.ValidationResults {
			if validationResult.Status == activities.Fail {
				failedPolicies = append(failedPolicies, validationResult.PolicySet)
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
