package event

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
)

type CheckRunAction interface {
	GetType() string
}

type WrappedCheckRunAction string

func (a WrappedCheckRunAction) GetType() string {
	return string(a)
}

type RequestedActionChecksAction struct {
	Identifier string
}

func (a RequestedActionChecksAction) GetType() string {
	return "requested_action"
}

type CheckRun struct {
	Action     CheckRunAction
	ExternalID string
}

type CheckRunHandler struct {
	Logger         logging.Logger
	TemporalClient signaler
}

func (h *CheckRunHandler) Handle(ctx context.Context, event CheckRun) error {
	if event.Action.GetType() != "requested_action" {
		h.Logger.Debug("ignoring checks event that isn't a requested action")
		return nil
	}

	action, ok := event.Action.(RequestedActionChecksAction)

	if !ok {
		return fmt.Errorf("event action type does not match string type.  This is likely a code bug")
	}

	status, err := toPlanReviewStatus(action)

	if err != nil {
		return errors.Wrap(err, "converting action to plan status")
	}

	err = h.TemporalClient.SignalWorkflow(
		ctx,
		event.ExternalID,
		// keeping this empty is fine since temporal will find the currently running workflow
		"",
		workflows.TerraformPlanReviewSignalName,
		workflows.TerraformPlanReviewSignalRequest{
			Status: status,
		})

	if err != nil {
		return errors.Wrapf(err, "signaling workflow with id: %s", event.ExternalID)
	}

	return nil

}

func toPlanReviewStatus(action RequestedActionChecksAction) (workflows.TerraformPlanReviewStatus, error) {
	switch action.Identifier {
	case "approve":
		return workflows.ApprovedPlanReviewStatus, nil
	case "reject":
		return workflows.RejectedPlanReviewStatus, nil
	}

	return workflows.RejectedPlanReviewStatus, fmt.Errorf("unknown action id %s", action.Identifier)
}
