package event

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	vcs_client "github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/sync"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/vcs"
	"go.temporal.io/sdk/client"
	"path/filepath"
)

type PushAction string

const (
	DeletedAction PushAction = "deleted"
	CreatedAction PushAction = "created"
	UpdatedAction PushAction = "updated"
)

type Push struct {
	Repo              models.Repo
	Ref               vcs.Ref
	Sha               string
	Sender            vcs.User
	InstallationToken int64
	Action            PushAction
}

type signaler interface {
	SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{},
		options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

type scheduler interface {
	Schedule(ctx context.Context, f sync.Executor) error
}

const defaultWorkspace = "default"
const repos = "repos"

type PushHandler struct {
	Allocator                     feature.Allocator
	Scheduler                     scheduler
	TemporalClient                signaler
	Logger                        logging.Logger
	GlobalCfg                     valid.GlobalCfg
	ProjectFinder                 events.ProjectFinder
	VCSClient                     vcs_client.Client
	PreWorkflowHooksCommandRunner events.PreWorkflowHooksCommandRunner
	ParserValidator               config.ParserValidator
	DataDir                       string
	WorkingDir                    events.WorkingDir
	AutoplanFileList              string
}

func (p *PushHandler) Handle(ctx context.Context, event Push) error {
	shouldAllocate, err := p.Allocator.ShouldAllocate(feature.PlatformMode, feature.FeatureContext{
		RepoName: event.Repo.FullName,
	})

	if err != nil {
		p.Logger.ErrorContext(ctx, "unable to allocate platformmode")
		return nil
	}

	if !shouldAllocate {
		p.Logger.DebugContext(ctx, "handler not configured for allocation")
		return nil
	}

	if event.Ref.Type != vcs.BranchRef || event.Ref.Name != event.Repo.DefaultBranch {
		p.Logger.DebugContext(ctx, "dropping event for unexpected ref")
		return nil
	}

	if event.Action == DeletedAction {
		p.Logger.WarnContext(ctx, "ref was deleted, resources might still exist")
		return nil
	}

	return p.Scheduler.Schedule(ctx, func(ctx context.Context) error {
		return p.handle(ctx, event)
	})
}

func (p *PushHandler) handle(ctx context.Context, event Push) error {
	projectCfgs, err := p.buildRoots(event, ctx)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}
	for _, projectCfg := range projectCfgs {
		run, err := p.startWorkflow(ctx, event, projectCfg)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}

		p.Logger.InfoContext(ctx, "Signaled workflow.", map[string]interface{}{
			"workflow-id": run.GetID(), "run-id": run.GetRunID(),
		})
	}
	return nil
}

func (p *PushHandler) startWorkflow(ctx context.Context, event Push, cfg *valid.MergedProjectCfg) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{TaskQueue: workflows.DeployTaskQueue}
	run, err := p.TemporalClient.SignalWithStartWorkflow(
		ctx,
		fmt.Sprintf("%s||%s", event.Repo.FullName, cfg.Name),
		workflows.DeployNewRevisionSignalID,
		workflows.DeployNewRevisionSignalRequest{
			Revision: event.Sha,
		},
		options,
		workflows.Deploy,

		// TODO: add other request params as we support them
		workflows.DeployRequest{
			Repository: workflows.Repo{
				URL:      event.Repo.CloneURL,
				FullName: event.Repo.FullName,
				Name:     event.Repo.Name,
				Owner:    event.Repo.Owner,
				Credentials: workflows.AppCredentials{
					InstallationToken: event.InstallationToken,
				},
			},
			Root: workflows.Root{
				Name: cfg.Name,
				Plan: workflows.Job{
					Steps: p.generateSteps(cfg.Workflow.Plan.Steps),
				},
				Apply: workflows.Job{
					Steps: p.generateSteps(cfg.Workflow.Apply.Steps),
				},
			},
		},
	)
	return run, err
}

func (p *PushHandler) generateSteps(steps []valid.Step) []workflows.Step {
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

func (p *PushHandler) buildRoots(event Push, ctx context.Context) ([]*valid.MergedProjectCfg, error) {
	err := p.PreWorkflowHooksCommandRunner.RunPreHooksWithSha(ctx, p.Logger, event.Repo, event.Sha)
	if err != nil {
		p.Logger.Error(fmt.Sprintf("Error running pre-workflow hooks %s. Proceeding with root building.", err))
	}

	modifiedFiles, err := p.VCSClient.GetModifiedFilesFromCommit(event.Repo, event.Sha)
	if err != nil {
		return nil, err
	}

	workspace := "default"
	repoDir, err := p.WorkingDir.CloneFromSha(p.Logger, event.Repo, event.Sha, workspace)
	if err != nil {
		return nil, err
	}

	// Parse config file if it exists.
	hasRepoCfg, err := p.ParserValidator.HasRepoCfg(repoDir)
	if err != nil {
		return nil, errors.Wrapf(err, "looking for %s file in %q", config.AtlantisYAMLFilename, repoDir)
	}

	var mergedProjectCfgs []*valid.MergedProjectCfg
	if hasRepoCfg {
		// If there's a repo cfg then we'll use it to figure out which projects
		// should be planed.
		repoCfg, err := p.ParserValidator.ParseRepoCfg(repoDir, p.GlobalCfg, event.Repo.ID())
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", config.AtlantisYAMLFilename)
		}
		matchingProjects, err := p.ProjectFinder.DetermineProjectsViaConfig(p.Logger, modifiedFiles, repoCfg, repoDir)
		if err != nil {
			return nil, err
		}

		for _, mp := range matchingProjects {
			mergedProjectCfg := p.GlobalCfg.MergeProjectCfg(p.Logger, event.Repo.ID(), mp, repoCfg)
			mergedProjectCfgs = append(mergedProjectCfgs, &mergedProjectCfg)
		}
	} else {
		// If there is no config file, then we'll plan each project that
		// our algorithm determines was modified.
		modifiedProjects := p.ProjectFinder.DetermineProjects(p.Logger, ctx, modifiedFiles, event.Repo.FullName, repoDir, p.AutoplanFileList)
		if err != nil {
			return nil, errors.Wrapf(err, "finding modified projects: %s", modifiedFiles)
		}
		for _, mp := range modifiedProjects {
			mergedProjectCfg := p.GlobalCfg.DefaultProjCfg(p.Logger, event.Repo.ID(), mp.Path, defaultWorkspace)
			mergedProjectCfgs = append(mergedProjectCfgs, &mergedProjectCfg)

		}
	}

	return mergedProjectCfgs, nil
}

func (p *PushHandler) cloneDir(r models.Repo, sha string, workspace string) string {
	return filepath.Join(p.DataDir, repos, r.FullName, sha, workspace)
}
