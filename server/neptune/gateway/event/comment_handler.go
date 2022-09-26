package event

import (
	"bytes"
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	contextInternal "github.com/runatlantis/atlantis/server/neptune/gateway/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"go.temporal.io/sdk/client"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

// Comment is our internal representation of a vcs based comment event.
type Comment struct {
	Pull              models.PullRequest
	BaseRepo          models.Repo
	HeadRepo          models.Repo
	User              models.User
	PullNum           int
	Comment           string
	VCSHost           models.VCSHostType
	Timestamp         time.Time
	InstallationToken int64
}

func NewCommentEventWorkerProxy(logger logging.Logger, snsWriter Writer, allocator feature.Allocator) *CommentEventWorkerProxy {
	return &CommentEventWorkerProxy{logger: logger, snsWriter: snsWriter, allocator: allocator}
}

type CommentEventWorkerProxy struct {
	logger            logging.Logger
	snsWriter         Writer
	allocator         feature.Allocator
	RootConfigBuilder rootConfigBuilder
	Scheduler         scheduler
	TemporalClient    signaler
}

func (p *CommentEventWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event Comment, command *command.Comment) error {
	shouldAllocate, err := p.allocator.ShouldAllocate(feature.PlatformMode, feature.FeatureContext{
		RepoName: event.BaseRepo.FullName,
	})

	if err != nil {
		p.logger.ErrorContext(ctx, "unable to allocate platform mode")
		return nil
	}

	if shouldAllocate && command.ForceApply {
		p.logger.WarnContext(ctx, "force apply comment")
		return p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
			return p.handleForceApplyComment(ctx, event)
		})
	}

	buffer := bytes.NewBuffer([]byte{})

	if err := request.GetRequestWithContext(ctx).Write(buffer); err != nil {
		return errors.Wrap(err, "writing request to buffer")
	}

	if err := p.snsWriter.WriteWithContext(ctx, buffer.Bytes()); err != nil {
		return errors.Wrap(err, "writing buffer to sns")
	}

	p.logger.InfoContext(ctx, "proxied request to sns")

	return nil
}

func (p *CommentEventWorkerProxy) handleForceApplyComment(ctx context.Context, event Comment) error {
	p.logger.WarnContext(ctx, fmt.Sprintf("building force apply: %s %s %d", event.BaseRepo.FullName, event.Pull.HeadCommit, event.InstallationToken))
	rootCfgs, err := p.RootConfigBuilder.Build(ctx, event.BaseRepo, event.Pull.HeadCommit, event.InstallationToken)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}
	for _, rootCfg := range rootCfgs {
		p.logger.WarnContext(ctx, fmt.Sprintf("starting workflow"))
		ctx = context.WithValue(ctx, contextInternal.ProjectKey, rootCfg.Name)
		run, err := p.startWorkflow(ctx, event, rootCfg)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}

		p.logger.InfoContext(ctx, "Signaled workflow.", map[string]interface{}{
			"workflow-id": run.GetID(), "run-id": run.GetRunID(),
		})
	}
	return nil
}

func (p *CommentEventWorkerProxy) startWorkflow(ctx context.Context, event Comment, rootCfg *valid.MergedProjectCfg) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{TaskQueue: workflows.DeployTaskQueue}

	var tfVersion string
	if rootCfg.TerraformVersion != nil {
		tfVersion = rootCfg.TerraformVersion.String()
	}

	run, err := p.TemporalClient.SignalWithStartWorkflow(
		ctx,
		fmt.Sprintf("%s||%s", event.BaseRepo.FullName, rootCfg.Name),
		workflows.DeployNewRevisionSignalID,
		workflows.DeployNewRevisionSignalRequest{
			Revision: event.Pull.HeadCommit,
		},
		options,
		workflows.Deploy,
		// TODO: add other request params as we support them
		workflows.DeployRequest{
			Repository: workflows.Repo{
				URL:      event.BaseRepo.CloneURL,
				FullName: event.BaseRepo.FullName,
				Name:     event.BaseRepo.Name,
				Owner:    event.BaseRepo.Owner,
				Credentials: workflows.AppCredentials{
					InstallationToken: event.InstallationToken,
				},
				HeadCommit: workflows.HeadCommit{
					Ref: workflows.Ref{
						Name: event.Pull.HeadRef.Name,
						Type: string(event.Pull.HeadRef.Type),
					},
				},
			},
			Root: workflows.Root{
				Name: rootCfg.Name,
				Plan: workflows.Job{
					Steps: p.generateSteps(rootCfg.DeploymentWorkflow.Plan.Steps),
				},
				Apply: workflows.Job{
					Steps: p.generateSteps(rootCfg.DeploymentWorkflow.Apply.Steps),
				},
				RepoRelPath: rootCfg.RepoRelDir,
				TfVersion:   tfVersion,
			},
		},
	)
	return run, err
}

func (p *CommentEventWorkerProxy) generateSteps(steps []valid.Step) []workflows.Step {
	// NOTE: for deployment workflows, we won't support command level user requests for log level output verbosity
	var workflowSteps []workflows.Step
	for _, step := range steps {
		workflowSteps = append(workflowSteps, workflows.Step{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}
