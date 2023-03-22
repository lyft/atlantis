package event

import (
	"bytes"
	"context"
	"github.com/runatlantis/atlantis/server/http"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	contextInternal "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"
	"go.temporal.io/sdk/client"
)

type deploySignaler interface {
	SignalWithStartWorkflow(ctx context.Context, rootCfg *valid.MergedProjectCfg, rootDeployOptions RootDeployOptions) (client.WorkflowRun, error)
	SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error
}

type rootConfigBuilder interface {
	Build(ctx context.Context, commit *RepoCommit, installationToken int64, opts ...BuilderOptions) ([]*valid.MergedProjectCfg, error)
}

type RootDeployer struct {
	Logger            logging.Logger
	RootConfigBuilder rootConfigBuilder
	DeploySignaler    deploySignaler
	SNSWriter         Writer
}

// RootDeployOptions is basically a modeled request for RootDeployer, options isn't really the right word here
type RootDeployOptions struct {
	Repo models.Repo

	// RootNames specify an optional list of roots to deploy for, if this is not provided, the roots are computed
	// via the configured fallback strategy.
	RootNames []string
	Branch    string
	Revision  string

	// By Specifying this, consumers can trigger deploys for all the roots modified in a PR if no roots are specified.
	// By default, computed roots are only based on the difference between the provided revision and revision - 1.
	OptionalPullNum int

	// User to attribute this deploy to
	Sender            models.User
	InstallationToken int64

	// TODO: Remove this from this struct, consumers shouldn't need to know about this
	// instead we should just inject implementations of RepoFetcher to handle different scenarios
	RepoFetcherOptions *github.RepoFetcherOptions
	Trigger            workflows.Trigger
	Rerun              bool

	// only used in force apply comments (if a force apply comment contains legacy mode roots)
	InitialRequest *http.BufferedRequest
}

func (d *RootDeployer) Deploy(ctx context.Context, deployOptions RootDeployOptions) error {
	commit := &RepoCommit{
		Repo:          deployOptions.Repo,
		Branch:        deployOptions.Branch,
		Sha:           deployOptions.Revision,
		OptionalPRNum: deployOptions.OptionalPullNum,
	}

	opts := BuilderOptions{
		RootNames:          deployOptions.RootNames,
		RepoFetcherOptions: deployOptions.RepoFetcherOptions,
	}

	rootCfgs, err := d.RootConfigBuilder.Build(ctx, commit, deployOptions.InstallationToken, opts)
	if err != nil {
		return errors.Wrap(err, "generating roots")
	}
	legacyRootFound := false
	for _, rootCfg := range rootCfgs {
		c := context.WithValue(ctx, contextInternal.ProjectKey, rootCfg.Name)
		if rootCfg.WorkflowMode != valid.PlatformWorkflowMode {
			d.Logger.DebugContext(c, "root is not configured for platform mode, skipping...")
			legacyRootFound = true
			continue
		}
		run, err := d.DeploySignaler.SignalWithStartWorkflow(c, rootCfg, deployOptions)
		if err != nil {
			return errors.Wrap(err, "signalling workflow")
		}

		d.Logger.InfoContext(c, "Signaled workflow.", map[string]interface{}{
			"workflow-id": run.GetID(), "run-id": run.GetRunID(),
		})
	}
	// Forward initial request to legacy worker if legacy roots are found
	if legacyRootFound {
		return d.forwardToSns(ctx, deployOptions.InitialRequest)
	}
	return nil
}

func (d *RootDeployer) forwardToSns(ctx context.Context, request *http.BufferedRequest) error {
	buffer := bytes.NewBuffer([]byte{})
	if err := request.GetRequestWithContext(ctx).Write(buffer); err != nil {
		return errors.Wrap(err, "writing request to buffer")
	}

	if err := d.SNSWriter.WriteWithContext(ctx, buffer.Bytes()); err != nil {
		return errors.Wrap(err, "writing buffer to sns")
	}
	d.Logger.InfoContext(ctx, "proxied request to sns")
	return nil
}
