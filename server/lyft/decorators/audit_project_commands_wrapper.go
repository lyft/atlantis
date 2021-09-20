package decorators

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/metrics"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/aws/sns"
)

// AuditProjectCommandWrapper is a decorator that notifies sns topic
// about the state of the command. It is used for auditing purposes
type AuditProjectCommandWrapper struct {
	SnsWriter sns.Writer
	events.ProjectCommandRunner
}

func (p *AuditProjectCommandWrapper) Apply(ctx models.ProjectCommandContext) models.ProjectResult {
	ctx.SetScope("audit_applies")

	id := uuid.New()
	startTime := time.Now()

	applyEvent := &ApplyEvent{
		Version:        1,
		ID:             id.String(),
		RootName:       ctx.ProjectName,
		JobType:        models.ApplyCommand,
		Respository:    ctx.BaseRepo.FullName,
		Environment:    ctx.Tags["environment"],
		PullNumber:     ctx.Pull.Num,
		InitiatingUser: ctx.User.Username,
		Project:        ctx.Tags["service_name"],
		ForceApply:     ctx.ForceApply,
		StartTime:      startTime.Format(time.RFC3339),
		Revision:       ctx.Pull.HeadCommit,
	}

	if err := p.emit(ctx, ApplyEventRunning, applyEvent); err != nil {
		ctx.Log.Err("failed to emit apply event", err)
	}

	result := p.ProjectCommandRunner.Apply(ctx)

	if result.Error != nil || result.Failure != "" {
		if err := p.emit(ctx, ApplyEventFailure, applyEvent); err != nil {
			ctx.Log.Err("failed to emit apply event", err)
		}

		return result
	}

	if err := p.emit(ctx, ApplyEventSuccess, applyEvent); err != nil {
		ctx.Log.Err("failed to emit apply event", err)
	}

	return result
}

func (p *AuditProjectCommandWrapper) emit(
	ctx models.ProjectCommandContext,
	state EventState,
	applyEvent *ApplyEvent,
) error {
	scope := ctx.Scope.Scope(state.String())

	applyEvent.State = state

	if state == ApplyEventFailure || state == ApplyEventSuccess {
		applyEvent.EndTime = time.Now().Format(time.RFC3339)
	}

	payload, err := applyEvent.Marshal()
	if err != nil {
		scope.NewCounter(metrics.ExecutionErrorMetric)

		return errors.Wrap(err, "marshaling apply event")
	}

	if err := p.SnsWriter.Write(payload); err != nil {
		scope.NewCounter(metrics.ExecutionErrorMetric)

		return errors.Wrap(err, "writing to sns topic")
	}

	scope.NewCounter(metrics.ExecutionSuccessMetric)

	return nil
}

// ApplyEvent contains metadata of the state of the apply command
type ApplyEvent struct {
	Version        int
	ID             string
	State          EventState
	JobType        models.CommandName
	Revision       string
	Respository    string
	PullNumber     int
	Environment    string
	InitiatingUser string
	StartTime      string
	EndTime        string
	ForceApply     bool

	// Service name in the manifest.yaml
	Project string
	// ProjectName in the atlantis.yaml
	RootName string

	// Currently we do not track approvers metadata.
	// ORCA-954 will implement this feature
	ApprovedBy   string
	ApprovedTime string
}

func (a *ApplyEvent) Marshal() ([]byte, error) {
	eventPayload, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling apply event")
	}

	return eventPayload, nil
}

type EventState int

const (
	ApplyEventRunning EventState = iota
	ApplyEventSuccess
	ApplyEventFailure
)

func (a EventState) String() string {
	switch a {
	case ApplyEventRunning:
		return "running"
	case ApplyEventSuccess:
		return "success"
	case ApplyEventFailure:
		return "failure"
	}

	return "unknown"
}
