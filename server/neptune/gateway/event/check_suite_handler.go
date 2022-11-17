package event

import (
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	contextInternal "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/vcs"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
)

type CheckSuite struct {
	Action            CheckRunAction
	HeadSha           string
	Repo              models.Repo
	Sender            models.User
	InstallationToken int64
	Branch            string
}

type CheckSuiteHandler struct {
	Logger            logging.Logger
	RootConfigBuilder rootConfigBuilder
	Scheduler         scheduler
	DeploySignaler    deploySignaler
}

func (h *CheckSuiteHandler) Handle(ctx context.Context, event CheckSuite) error {
	if event.Action.GetType() != ReRequestedActionType {
		h.Logger.DebugContext(ctx, "ignoring checks event that isn't a rerequested action")
		return nil
	}
	// Block force applies
	if event.Branch != event.Repo.DefaultBranch {
		h.Logger.DebugContext(ctx, "dropping event branch unexpected ref")
		return nil
	}
	return h.Scheduler.Schedule(ctx, func(ctx context.Context) error {
		return h.handle(ctx, event)
	})
}

func (h *CheckSuiteHandler) handle(ctx context.Context, event CheckSuite) error {
	builderOptions := BuilderOptions{
		RepoFetcherOptions: github.RepoFetcherOptions{
			ShallowClone: true,
		},
		FileFetcherOptions: github.FileFetcherOptions{
			Sha: event.HeadSha,
		},
	}
	rootCfgs, err := h.RootConfigBuilder.Build(ctx, event.Repo, event.Branch, event.HeadSha, event.InstallationToken, builderOptions)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}
	for _, rootCfg := range rootCfgs {
		c := context.WithValue(ctx, contextInternal.ProjectKey, rootCfg.Name)

		if rootCfg.WorkflowMode != valid.PlatformWorkflowMode {
			h.Logger.DebugContext(c, "root is not configured for platform mode, skipping...")
			continue
		}

		run, err := h.DeploySignaler.SignalWithStartWorkflow(
			c,
			rootCfg,
			event.Repo,
			event.HeadSha,
			event.InstallationToken,
			vcs.Ref{
				Type: vcs.BranchRef,
				Name: event.Branch,
			},
			event.Sender,
			workflows.MergeTrigger)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}

		h.Logger.InfoContext(c, "Signaled workflow.", map[string]interface{}{
			"workflow-id": run.GetID(), "run-id": run.GetRunID(),
		})
	}
	return nil
}
