package runtime

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
)

// NullRunner is a runner that isn't configured for a given plan type but outputs nothing
type NullRunner struct{}

func (p NullRunner) Run(ctx context.Context, prjCtx command.ProjectContext, extraArgs []string, path string, envs map[string]string) (string, error) {
	prjCtx.Log.Debugf("runner not configured for plan type")

	return "", nil
}

// RemoteBackendUnsupportedRunner is a runner that is responsible for outputting that the remote backend is unsupported
type RemoteBackendUnsupportedRunner struct{}

func (p RemoteBackendUnsupportedRunner) Run(ctx context.Context, cmdCtx command.ProjectContext, extraArgs []string, path string, envs map[string]string) (string, error) {
	cmdCtx.Log.Debugf("runner not configured for remote backend")

	return "Remote backend is unsupported for this step.", nil
}

func NewPlanTypeStepRunnerDelegate(defaultRunner Runner, remotePlanRunner Runner) Runner {
	return &PlanTypeStepRunnerDelegate{
		defaultRunner:    defaultRunner,
		remotePlanRunner: remotePlanRunner,
	}
}

// PlanTypeStepRunnerDelegate delegates based on the type of plan, ie. remote backend which doesn't support certain functions
type PlanTypeStepRunnerDelegate struct {
	defaultRunner    Runner
	remotePlanRunner Runner
}

func (p *PlanTypeStepRunnerDelegate) isRemotePlan(planFile string) (bool, error) {
	data, err := ioutil.ReadFile(planFile)

	if err != nil {
		return false, errors.Wrapf(err, "unable to read %s", planFile)
	}

	return IsRemotePlan(data), nil
}

func (p *PlanTypeStepRunnerDelegate) Run(ctx context.Context, cmdCtx command.ProjectContext, extraArgs []string, path string, envs map[string]string) (string, error) {
	planFile := filepath.Join(path, GetPlanFilename(cmdCtx.Workspace, cmdCtx.ProjectName))
	remotePlan, err := p.isRemotePlan(planFile)

	if err != nil {
		return "", err
	}

	if remotePlan {
		return p.remotePlanRunner.Run(ctx, cmdCtx, extraArgs, path, envs)
	}

	return p.defaultRunner.Run(ctx, cmdCtx, extraArgs, path, envs)
}
