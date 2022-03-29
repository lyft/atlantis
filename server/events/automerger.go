package events

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

type AutoMerger struct {
	VCSClient       vcs.Client
	GlobalAutomerge bool
}

func (c *AutoMerger) automerge(ctx *command.Context, pullStatus models.PullStatus, deleteSourceBranchOnMerge bool) {
	// We only automerge if all projects have been successfully applied.
	for _, p := range pullStatus.Projects {
		if p.Status != models.AppliedPlanStatus {
			ctx.Log.Infof("not automerging because project at dir %q, workspace %q has status %q", p.RepoRelDir, p.Workspace, p.Status.String())
			return
		}
	}

	// Comment that we're automerging the pull request.
	if err := c.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, automergeComment, command.Apply.String()); err != nil {
		ctx.Log.Errorf("failed to comment about automerge: %s", err)
		// Commenting isn't required so continue.
	}

	// Make the API call to perform the merge.
	ctx.Log.Infof("automerging pull request")
	var pullOptions models.PullRequestOptions
	pullOptions.DeleteSourceBranchOnMerge = deleteSourceBranchOnMerge
	err := c.VCSClient.MergePull(ctx.Pull, pullOptions)

	if err != nil {
		ctx.Log.Errorf("automerging failed: %s", err)

		failureComment := fmt.Sprintf("Automerging failed:\n```\n%s\n```", err)
		if commentErr := c.VCSClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, failureComment, command.Apply.String()); commentErr != nil {
			ctx.Log.Errorf("failed to comment about automerge failing: %s", err)
		}
	}
}

// AutomergeEnabled returns true if automerging is enabled in this context.
func (c *AutoMerger) automergeEnabled(projectCmds []command.ProjectContext) bool {
	// If the global automerge is set, we always automerge.
	return c.GlobalAutomerge ||
		// Otherwise we check if this repo is configured for automerging.
		(len(projectCmds) > 0 && projectCmds[0].AutomergeEnabled)
}

// deleteSourceBranchOnMergeEnabled returns true if we should delete the source branch on merge in this context.
func (c *AutoMerger) deleteSourceBranchOnMergeEnabled(projectCmds []command.ProjectContext) bool {
	//check if this repo is configured for automerging.
	return (len(projectCmds) > 0 && projectCmds[0].DeleteSourceBranchOnMerge)
}
