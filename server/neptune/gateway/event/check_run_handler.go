package event

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
)

const RequestedActionType = "requested_action"

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
	return RequestedActionType
}

type CheckRun struct {
	Action     CheckRunAction
	ExternalID string
	Name       string
	Repo       models.Repo
	User       string
}

type CheckRunHandler struct {
	Logger         logging.Logger
	TemporalClient signaler
}

func (h *CheckRunHandler) Handle(ctx context.Context, event CheckRun) error {
	// first let's make sure this is an atlantis check run
	if !strings.HasPrefix(event.Name, "atlantis") {
		h.Logger.DebugContext(ctx, "Ignoring non-atlantis checks event")
		return nil
	}

	// we only handle requested action types
	if event.Action.GetType() != RequestedActionType {
		h.Logger.DebugContext(ctx, "ignoring checks event that isn't a requested action")
		return nil
	}

	action, ok := event.Action.(RequestedActionChecksAction)
	if !ok {
		return fmt.Errorf("event action type does not match string type.  This is likely a code bug")
	}
	switch action.Identifier {
	case "Unlock":
		return h.signalUnlockWorkflowChannel(ctx, event)
	case "Approve":
		return h.signalPlanReviewWorkflowChannel(ctx, event, workflows.ApprovedPlanReviewStatus)
	case "Reject":
		return h.signalPlanReviewWorkflowChannel(ctx, event, workflows.RejectedPlanReviewStatus)
	}
	return fmt.Errorf("unknown action id %s", action.Identifier)
}

func (h *CheckRunHandler) signalPlanReviewWorkflowChannel(ctx context.Context, event CheckRun, status workflows.TerraformPlanReviewStatus) error {
	err := h.TemporalClient.SignalWorkflow(
		ctx,
		// assumed that we're using the check run external id as our workflow id
		event.ExternalID,
		// keeping this empty is fine since temporal will find the currently running workflow
		"",
		workflows.TerraformPlanReviewSignalName,
		workflows.TerraformPlanReviewSignalRequest{
			Status: status,
			User:   event.User,
		})
	if err != nil {
		return errors.Wrapf(err, "signaling workflow with id: %s", event.ExternalID)
	}
	h.Logger.InfoContext(ctx, fmt.Sprintf("Signaled workflow with id %s, review status, %d", event.ExternalID, status))
	return nil
}

func (h *CheckRunHandler) signalUnlockWorkflowChannel(ctx context.Context, event CheckRun) error {
	err := h.TemporalClient.SignalWorkflow(
		ctx,
		// assumed that we're using the check run external id as our workflow id
		event.ExternalID,
		// keeping this empty is fine since temporal will find the currently running workflow
		"",
		workflows.DeployUnlockSignalName,
		workflows.DeployUnlockSignalRequest{
			User: event.User,
		})
	if err != nil {
		return errors.Wrapf(err, "signaling workflow with id: %s", event.ExternalID)
	}
	h.Logger.InfoContext(ctx, fmt.Sprintf("Signaled workflow with id %s to unlock", event.ExternalID))
	return nil
}
