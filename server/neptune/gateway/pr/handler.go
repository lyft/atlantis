package pr

import (
	"context"
	"github.com/runatlantis/atlantis/server/neptune/gateway/config"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/logging"
	contextInternal "github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/sdk/client"
)

type prSignaler interface {
	SignalWithStartWorkflow(ctx context.Context, rootCfgs []*valid.MergedProjectCfg, prOptions Options) (client.WorkflowRun, error)
}

type rootConfigBuilder interface {
	Build(ctx context.Context, commit *config.RepoCommit, installationToken int64, opts ...config.BuilderOptions) ([]*valid.MergedProjectCfg, error)
}

type RevisionHandler struct {
	Logger            logging.Logger
	GlobalCfg         valid.GlobalCfg
	RootConfigBuilder rootConfigBuilder
	PRSignaler        prSignaler
}

func (h *RevisionHandler) Handle(ctx context.Context, prOptions Options) error {
	commit := &config.RepoCommit{
		Repo:          prOptions.Repo,
		Branch:        prOptions.Branch,
		Sha:           prOptions.Revision,
		OptionalPRNum: prOptions.Number,
	}

	// set clone depth to 1 for repos with a branch checkout strategy
	cloneDepth := -1
	matchingRepo := h.GlobalCfg.MatchingRepo(prOptions.Repo.ID())
	if matchingRepo != nil && matchingRepo.CheckoutStrategy == "branch" {
		cloneDepth = 1
	}
	builderOptions := config.BuilderOptions{
		RepoFetcherOptions: &github.RepoFetcherOptions{
			CloneDepth: cloneDepth,
		},
	}

	rootCfgs, err := h.RootConfigBuilder.Build(ctx, commit, prOptions.InstallationToken, builderOptions)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}

	if len(rootCfgs) == 0 {
		// todo: mark atlantis as successful?
		return nil
	}

	var platformModeRoots []*valid.MergedProjectCfg
	for _, rootCfg := range rootCfgs {
		c := context.WithValue(ctx, contextInternal.ProjectKey, rootCfg.Name)
		if rootCfg.WorkflowMode != valid.PlatformWorkflowMode {
			h.Logger.WarnContext(c, "root is not configured for platform mode, skipping...")
			continue
		}
		platformModeRoots = append(platformModeRoots, rootCfg)
	}
	run, err := h.PRSignaler.SignalWithStartWorkflow(ctx, platformModeRoots, prOptions)
	if err != nil {
		return errors.Wrap(err, "signaling workflow")
	}
	h.Logger.InfoContext(ctx, "Signaled workflow.", map[string]interface{}{
		"workflow-id": run.GetID(), "run-id": run.GetRunID(),
	})
	return nil
}
