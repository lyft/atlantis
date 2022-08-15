package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

const KeySeparator = "||"

type PolicyCheckOutputStore struct {
	Store map[string]*models.PolicyCheckSuccess
}

func NewPolicyCheckOutputStore() *PolicyCheckOutputStore {
	return &PolicyCheckOutputStore{
		Store: map[string]*models.PolicyCheckSuccess{},
	}
}

func (p *PolicyCheckOutputStore) buildKey(projectName string, workspace string) string {
	return fmt.Sprintf("%s%s%s", projectName, KeySeparator, workspace)
}

func (p *PolicyCheckOutputStore) GetOutputFor(projectName string, workspace string) *models.PolicyCheckSuccess {
	key := p.buildKey(projectName, workspace)

	if output, ok := p.Store[key]; ok {
		return output
	}
	return nil
}

func (p *PolicyCheckOutputStore) WriteOutputFor(projectName string, workspace string, policyCheckResult command.ProjectResult) {
	key := p.buildKey(projectName, workspace)

	var output string
	if policyCheckResult.Failure != "" {
		output = policyCheckResult.Failure
	} else if policyCheckResult.PolicyCheckSuccess != nil {
		output = policyCheckResult.PolicyCheckSuccess.PolicyCheckOutput
	}

	val := &models.PolicyCheckSuccess{
		PolicyCheckOutput: output,
	}

	p.Store[key] = val
}

type CommandOutputGenerator interface {
	GenerateCommandOutput(ctx *command.Context, cmd *command.Comment) PolicyCheckOutputStore
}

type PolicyCheckCommandOutputGenerator struct {
	PrjCommandRunner  ProjectPolicyCheckCommandRunner
	PrjCommandBuilder ProjectPlanCommandBuilder
	FeatureAllocator  feature.Allocator
}

func (f *PolicyCheckCommandOutputGenerator) GenerateCommandOutput(ctx *command.Context, cmd *command.Comment) PolicyCheckOutputStore {
	if !f.isChecksEnabled(ctx) {
		return PolicyCheckOutputStore{}
	}

	prjCmds, err := f.PrjCommandBuilder.BuildPlanCommands(ctx, &command.Comment{
		RepoRelDir:  cmd.RepoRelDir,
		Name:        command.Plan,
		Workspace:   cmd.Workspace,
		ProjectName: cmd.ProjectName,
		LogLevel:    cmd.LogLevel,
	})
	if err != nil {
		ctx.Log.WarnContext(ctx.RequestCtx, fmt.Sprintf("unable to build plan command: %s", err))
		return PolicyCheckOutputStore{}
	}

	policyCheckCommands := f.getPolicyCheckCommands(ctx, prjCmds)
	policyCheckOutputStore := NewPolicyCheckOutputStore()
	for _, policyCheckCmd := range policyCheckCommands {
		policyCheckResult := f.PrjCommandRunner.PolicyCheck(policyCheckCmd)
		policyCheckOutputStore.WriteOutputFor(policyCheckCmd.ProjectName, policyCheckCmd.Workspace, policyCheckResult)
	}
	return *policyCheckOutputStore
}

func (f *PolicyCheckCommandOutputGenerator) getPolicyCheckCommands(
	ctx *command.Context,
	cmds []command.ProjectContext,
) []command.ProjectContext {
	policyCheckCmds := []command.ProjectContext{}
	for _, cmd := range cmds {
		if cmd.CommandName == command.PolicyCheck {
			policyCheckCmds = append(policyCheckCmds, cmd)
		}
	}
	return policyCheckCmds
}

func (f *PolicyCheckCommandOutputGenerator) isChecksEnabled(ctx *command.Context) bool {
	shouldAllocate, err := f.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         ctx.HeadRepo.Name,
		PullCreationTime: ctx.Pull.CreatedAt,
	})
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("unable to allocate for feature: %s, error: %s", feature.GithubChecks, err))
		return false
	}

	return shouldAllocate
}
