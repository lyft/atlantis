package policy

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/pr/revision"
	temporalInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

type dismisser interface {
	Dismiss(ctx workflow.Context, revision revision.Revision) error
}

type policyFilter interface {
	Filter(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) ([]activities.PolicySet, error)
}

type NewApproveSignalRequest struct{}

type FailedPolicyHandler struct {
	ApprovalSignalChannel workflow.ReceiveChannel
	Dismisser             dismisser
	PolicyFilter          policyFilter
}

type Action int64

const (
	onApprovalSignal Action = iota
	onPollTick
)

func (f *FailedPolicyHandler) Handle(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) {
	policies := dedup(failedPolicies)

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
		if len(policies) == 0 {
			break
		}
		s.Select(ctx)
		if action == onPollTick {
			cancelTimer()
			cancelTimer, _ = s.AddTimeout(ctx, 10*time.Minute, onTimeout)
		}
		policies = f.handle(ctx, revision, policies)
	}
}

func (f *FailedPolicyHandler) handle(ctx workflow.Context, revision revision.Revision, failedPolicies []activities.PolicySet) []activities.PolicySet {
	// Dismiss stale approvals
	err := f.Dismisser.Dismiss(ctx, revision)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to dismiss stale reviews")
		return nil
	}

	// Filter out failed policies from policy approvals
	filteredPolicies, err := f.PolicyFilter.Filter(ctx, revision, failedPolicies)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to dismiss stale reviews")
		return failedPolicies
	}
	return filteredPolicies
}

func dedup(failedPolicies []activities.PolicySet) []activities.PolicySet {
	uniquePolicies := make(map[string]activities.PolicySet)
	for _, failedPolicy := range failedPolicies {
		uniquePolicies[failedPolicy.Name] = failedPolicy
	}
	return toSlice(uniquePolicies)
}

func toSlice(policyMap map[string]activities.PolicySet) []activities.PolicySet {
	var policies []activities.PolicySet
	for _, policy := range policyMap {
		policies = append(policies, policy)
	}
	return policies
}
