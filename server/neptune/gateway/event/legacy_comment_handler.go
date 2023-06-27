package event

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
)

type LegacyCommentHandler struct {
	logger    logging.Logger
	snsWriter Writer
	globalCfg valid.GlobalCfg
}

func (p *LegacyCommentHandler) Handle(ctx context.Context, event Comment, cmd *command.Comment, roots []*valid.MergedProjectCfg, request *http.BufferedRequest) error {
	// legacy mode should not be handling any type of apply command anymore
	if cmd.Name == command.Apply {
		return nil
	}
	// forward everything to sns for now since platform mode doesn't do anything w.r.t to comments atm.
	if err := p.ForwardToSns(ctx, request); err != nil {
		return errors.Wrap(err, "forwarding request through sns")
	}
	return nil
}

func (p *LegacyCommentHandler) shouldMarkEventQueued(event Comment, cmd *command.Comment) bool {
	// pending status should only be for plan step
	if cmd.Name != command.Plan {
		return false
	}
	// pull event should not be from a fork
	if event.Pull.HeadRepo.Owner != event.Pull.BaseRepo.Owner {
		return false
	}
	// pull event should not be from closed PR
	if event.Pull.State == models.ClosedPullState {
		return false
	}
	// pull event should not use an invalid base branch
	repo := p.globalCfg.MatchingRepo(event.Pull.BaseRepo.ID())
	return repo.BranchMatches(event.Pull.BaseBranch)
}

func (p *LegacyCommentHandler) ForwardToSns(ctx context.Context, request *http.BufferedRequest) error {
	buffer := bytes.NewBuffer([]byte{})
	if err := request.GetRequestWithContext(ctx).Write(buffer); err != nil {
		return errors.Wrap(err, "writing request to buffer")
	}

	if err := p.snsWriter.WriteWithContext(ctx, buffer.Bytes()); err != nil {
		return errors.Wrap(err, "writing buffer to sns")
	}
	p.logger.InfoContext(ctx, "proxied request to sns")

	return nil
}
