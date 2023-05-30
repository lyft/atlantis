package policy

import (
	"context"
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform"
	"go.temporal.io/sdk/workflow"
	"time"
)

type githubActivities interface {
	GithubListPRApprovals(ctx context.Context, request activities.ListPRApprovalsRequest) (activities.ListPRApprovalsResponse, error)
}

type dismisser interface {
	Dismiss(ctx workflow.Context, revision revision.Revision, currentApprovals []*github.PullRequestReview) ([]*github.PullRequestReview, error)
}

type policyFilter interface {
	Filter(ctx workflow.Context, revision revision.Revision, currentApprovals []*github.PullRequestReview, failedPolicies []activities.PolicySet) ([]activities.PolicySet, error)
}

type NewApproveSignalRequest struct{}

type FailedPolicyHandler struct {
	ApprovalSignalChannel workflow.ReceiveChannel
	Dismisser             dismisser
	PolicyFilter          policyFilter
	GithubActivities      githubActivities
	PRNumber              int
}

type Action int64

const (
	onApprovalSignal Action = iota
	onPollTick
)

func (f *FailedPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, workflowResponses []terraform.Response) {
	failedPolicies := dedup(workflowResponses)
	if len(failedPolicies) == 0 {
		return
	}

	var action Action
	s := temporalInternal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}
	s.AddReceive(f.ApprovalSignalChannel, func(c workflow.ReceiveChannel, more bool) {
		defer func() {
			action = onApprovalSignal
		}()
		if !more {
			return
		}
		var request NewApproveSignalRequest
		c.Receive(ctx, &request)
	})
	onTimeout := func(f workflow.Future) {
		_ = f.Get(ctx, nil)
		action = onPollTick
	}
	cancelTimer, _ := s.AddTimeout(ctx, 5*time.Minute, onTimeout)

	for {
		if len(failedPolicies) == 0 {
			break
		}
		s.Select(ctx)
		if action == onPollTick {
			cancelTimer()
			// TODO: evaluate a better polling rate for approvals, or remove all together
			cancelTimer, _ = s.AddTimeout(ctx, 10*time.Minute, onTimeout)
		}
		failedPolicies = f.handle(ctx, revision, failedPolicies)
	}
}

func (f *FailedPolicyHandler) handle(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) []activities.PolicySet {
	// Fetch current approvals in activity
	var listPRApprovalsResponse activities.ListPRApprovalsResponse
	err := workflow.ExecuteActivity(ctx, f.GithubActivities.GithubListPRApprovals, activities.ListPRApprovalsRequest{
		Repo:     revision.Repo,
		PRNumber: f.PRNumber,
	}).Get(ctx, &listPRApprovalsResponse)
	if err != nil {
		workflow.GetLogger(ctx).Error(err.Error())
		return failedPolicies
	}

	// Dismiss stale approvals
	currentApprovals := listPRApprovalsResponse.Approvals
	currentApprovals, err = f.Dismisser.Dismiss(ctx, revision, currentApprovals)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to dismiss stale reviews")
		return failedPolicies
	}

	// Filter out failed policies from policy approvals
	filteredPolicies, err := f.PolicyFilter.Filter(ctx, revision, currentApprovals, failedPolicies)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to dismiss stale reviews")
		return failedPolicies
	}
	return filteredPolicies
}

func dedup(workflowResponses []terraform.Response) []activities.PolicySet {
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
