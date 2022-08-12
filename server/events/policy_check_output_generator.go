package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type PolicyCheckOutputKey struct {
	ProjectName string
	Workspace   string
}

type PolicyCheckOutputStore struct {
	Store map[PolicyCheckOutputKey]*models.PolicyCheckSuccess
}

func NewPolicyCheckOutputStore() *PolicyCheckOutputStore {
	return &PolicyCheckOutputStore{
		Store: map[PolicyCheckOutputKey]*models.PolicyCheckSuccess{},
	}
}

func (p *PolicyCheckOutputStore) GetOutputFor(projectName string, workspace string) *models.PolicyCheckSuccess {
	key := PolicyCheckOutputKey{
		ProjectName: projectName,
		Workspace:   workspace,
	}

	if ouptut, ok := p.Store[key]; ok {
		return ouptut
	}
	return nil
}

func (p *PolicyCheckOutputStore) WriteOutputFor(projectName string, workspace string, policyCheckResult command.ProjectResult) {

	key := PolicyCheckOutputKey{
		ProjectName: projectName,
		Workspace:   workspace,
	}

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

type NoopCommandOutputGenerator struct{}

func (n *NoopCommandOutputGenerator) GenerateCommandOutput(ctx *command.Context, cmd *command.Comment) PolicyCheckOutputStore {
	return PolicyCheckOutputStore{}
}

type PolicyCheckCommandOutputGenerator struct {
	PrjCommandRunner  ProjectPolicyCheckCommandRunner
	PrjCommandBuilder ProjectPlanCommandBuilder
	FeatureAllocator  feature.Allocator
}

func (f *PolicyCheckCommandOutputGenerator) GenerateCommandOutput(ctx *command.Context, cmd *command.Comment) PolicyCheckOutputStore {
	if !f.isChecksEnabled(ctx, ctx.HeadRepo, ctx.Pull) {
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

func (f *PolicyCheckCommandOutputGenerator) isChecksEnabled(ctx *command.Context, repo models.Repo, pull models.PullRequest) bool {
	shouldAllocate, err := f.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         repo.FullName,
		PullCreationTime: pull.CreatedAt,
	})
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("unable to allocate for feature: %s, error: %s", feature.GithubChecks, err))
		return false
	}

	return shouldAllocate
}
