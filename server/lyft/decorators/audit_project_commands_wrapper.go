package decorators

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
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

	if err := p.emit(ctx, ApplyEventInitiated, applyEvent); err != nil {
		ctx.Log.Err("failed to emit apply event", err)
	}

	result := p.ProjectCommandRunner.Apply(ctx)

	if result.Error != nil || result.Failure != "" {
		if err := p.emit(ctx, ApplyEventError, applyEvent); err != nil {
			ctx.Log.Err("failed to emit apply event", err)
		}

		return result
	}

	if err := p.emit(ctx, ApplyEventSuccess, applyEvent); err != nil {
		ctx.Log.Err("failed to emit apply event", err)
	}

	ctx.Log.Info("sns topic has been updated with the apply state")

	return result
}

func (p *AuditProjectCommandWrapper) emit(
	ctx models.ProjectCommandContext,
	state EventState,
	applyEvent *ApplyEvent,
) error {
	applyEvent.State = state

	if state == ApplyEventError || state == ApplyEventSuccess {
		applyEvent.EndTime = time.Now().Format(time.RFC3339)
	}

	payload, err := applyEvent.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshaling apply event")
	}

	if err := p.SnsWriter.Write(payload); err != nil {
		return errors.Wrap(err, "writing to sns topic")
	}

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
	ApplyEventInitiated EventState = iota
	ApplyEventSuccess
	ApplyEventError
)

func (a EventState) String() string {
	switch a {
	case ApplyEventInitiated:
		return "initiated"
	case ApplyEventSuccess:
		return "success"
	case ApplyEventError:
		return "error"
	}

	return "unknown"
}
