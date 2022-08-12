package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

type CommandOutputPopulator interface {
	PopulateCommandOutput(ctx *command.Context, cmd *command.Comment, result command.Result) command.Result
}

type NoopCommandOutputPopulator struct{}

func (n *NoopCommandOutputPopulator) PopulateCommandOutput(ctx *command.Context, cmd *command.Comment, result command.Result) command.Result {
	return result
}

type PolicyCheckCommandOutputPopulator struct {
	PrjCommandRunner  ProjectPolicyCheckCommandRunner
	PrjCommandBuilder ProjectPlanCommandBuilder
	FeatureAllocator  feature.Allocator
}

func (f *PolicyCheckCommandOutputPopulator) PopulateCommandOutput(ctx *command.Context, cmd *command.Comment, result command.Result) command.Result {
	if !f.isChecksEnabled(ctx, ctx.HeadRepo, ctx.Pull) {
		return result
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
		return result
	}

	policyCheckCommands := f.getPolicyCheckCommands(ctx, prjCmds)
	return f.populateOutputForFailingPolicies(ctx, policyCheckCommands, result)
}

func (f *PolicyCheckCommandOutputPopulator) populateOutputForFailingPolicies(ctx *command.Context, policyCheckCommands []command.ProjectContext, result command.Result) command.Result {
	for _, policyCheckCommand := range policyCheckCommands {
		res := f.PrjCommandRunner.PolicyCheck(policyCheckCommand)

		// Skip if policy check is success
		if res.PolicyCheckSuccess != nil {
			continue
		}

		for i, prjResult := range result.ProjectResults {
			if prjResult.ProjectName == policyCheckCommand.ProjectName &&
				prjResult.Workspace == policyCheckCommand.Workspace {
				result.ProjectResults[i].PolicyCheckSuccess = &models.PolicyCheckSuccess{
					PolicyCheckOutput: res.Failure,
				}
			}
		}
	}

	return result
}

func (f *PolicyCheckCommandOutputPopulator) getPolicyCheckCommands(
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

func (f *PolicyCheckCommandOutputPopulator) isChecksEnabled(ctx *command.Context, repo models.Repo, pull models.PullRequest) bool {
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
