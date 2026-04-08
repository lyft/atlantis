package gate

import (
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	PlanReviewSignalName = "planreview"
	PlanReviewTimerStat  = "workflow.terraform.planreview"
)

type PlanStatus int
type PlanReviewSignalRequest struct {
	Status PlanStatus
	User   string
}

const (
	Approved PlanStatus = iota
	Rejected
)

// ReviewResult holds the outcome of a plan review gate.
type ReviewResult struct {
	Status       PlanStatus
	ApprovedBy   string
	ApprovedTime time.Time
}

// Review waits for a plan review signal or a timeout to occur and returns an associated status.
type Review struct {
	MetricsHandler client.MetricsHandler
	Timeout        time.Duration
	Client         ActionsClient
}

type ActionsClient interface {
	UpdateApprovalActions(approval terraform.PlanApproval) error
}

// Await blocks until the plan is approved, rejected, or times out.
// On approval it returns the approver username and the time of approval.
func (r *Review) Await(ctx workflow.Context, root terraform.Root, planSummary terraform.PlanSummary) (ReviewResult, error) {
	if root.Plan.Approval.Type == terraform.AutoApproval || planSummary.IsEmpty() {
		return ReviewResult{Status: Approved}, nil
	}

	waitStartTime := time.Now()
	defer func() {
		r.MetricsHandler.Timer(PlanReviewTimerStat).Record(time.Since(waitStartTime))
	}()

	ch := workflow.GetSignalChannel(ctx, PlanReviewSignalName)
	selector := temporal.SelectorWithTimeout{
		Selector: workflow.NewSelector(ctx),
	}

	var planReview PlanReviewSignalRequest
	var approvedAt time.Time
	selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
		ch.Receive(ctx, &planReview)
		approvedAt = workflow.Now(ctx)
	})

	var timedOut bool
	selector.AddTimeout(ctx, r.Timeout, func(f workflow.Future) {
		if err := f.Get(ctx, nil); err != nil {
			workflow.GetLogger(ctx).Warn("Error timing out selector.  This is possibly due to a cancellation signal. ", context.ErrKey, err)
		}
		timedOut = true
	})

	err := r.Client.UpdateApprovalActions(root.Plan.Approval)
	if err != nil {
		return ReviewResult{Status: Rejected}, errors.Wrap(err, "updating approval actions")
	}

	selector.Select(ctx)

	if timedOut {
		return ReviewResult{Status: Rejected}, nil
	}

	return ReviewResult{
		Status:       planReview.Status,
		ApprovedBy:   planReview.User,
		ApprovedTime: approvedAt,
	}, nil
}
