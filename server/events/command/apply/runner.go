package apply

import (
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/vcs"
)

func NewDisabledRunner(vcsClient vcs.Client) *DisabledRunner {
	return &DisabledRunner{
		vcsClient: vcsClient,
	}
}

type DisabledRunner struct {
	vcsClient vcs.Client
}

func (r *DisabledRunner) Run(ctx *command.Context, cmd *command.Comment) {
	if err := r.vcsClient.CreateComment(ctx.Pull.BaseRepo, ctx.Pull.Num, "To Apply your changed merge the PR", command.Apply.String()); err != nil {
		ctx.Log.Errorf("unable to comment: %s", err)
	}
}
