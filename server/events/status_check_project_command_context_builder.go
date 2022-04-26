package events

import (
	"context"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

// StatusCheckCommandContextBuilder creates a status check for the project
type StatusCheckCommandContextBuilder struct {
	ProjectCommandContextBuilder
	CommitStatusUpdater
}

func (cb *StatusCheckCommandContextBuilder) BuildProjectContext(
	ctx *command.Context,
	cmdName command.Name,
	prjCfg valid.MergedProjectCfg,
	commentFlags []string,
	repoDir string,
	contextFlags *command.ContextFlags,
) (projectCmds []command.ProjectContext) {
	cmds := cb.ProjectCommandContextBuilder.BuildProjectContext(
		ctx, cmdName, prjCfg, commentFlags, repoDir, contextFlags,
	)

	for _, cmd := range cmds {
		statusID, _ := cb.CommitStatusUpdater.CreateProjectStatus(context.TODO(), cmd, cmdName, models.PendingCommitStatus)
		cmd.StatusID = statusID
	}
	return projectCmds
}
