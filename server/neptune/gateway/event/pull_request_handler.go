package event

import (
	"bytes"
	"context"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

type revisionHandler interface {
	Handle(ctx context.Context, options pr.Options) error
}

// PullRequest is our internal representation of a vcs based pr event
type PullRequest struct {
	Pull              models.PullRequest
	User              models.User
	EventType         models.PullRequestEventType
	Timestamp         time.Time
	InstallationToken int64
}

func NewModifiedPullHandler(
	autoplanValidator Validator,
	logger logging.Logger,
	workerProxy *PullSNSWorkerProxy,
	scheduler scheduler,
	revisonHandler revisionHandler,
) *ModifiedPullHandler {
	return &ModifiedPullHandler{
		autoplanValidator: autoplanValidator,
		workerProxy:       workerProxy,
		logger:            logger,
		scheduler:         scheduler,
		revisionHandler:   revisonHandler,
	}
}

type Validator interface {
	InstrumentedIsValid(ctx context.Context, logger logging.Logger, baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User) bool
}

type Writer interface {
	WriteWithContext(ctx context.Context, payload []byte) error
}

type ModifiedPullHandler struct {
	autoplanValidator Validator
	workerProxy       *PullSNSWorkerProxy
	logger            logging.Logger
	scheduler         scheduler
	allocator         feature.Allocator
	revisionHandler   revisionHandler
}

func (p *ModifiedPullHandler) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	return p.scheduler.Schedule(ctx, func(ctx context.Context) error {
		return p.handle(ctx, request, event)
	})
}

func (p *ModifiedPullHandler) handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
	// Run legacy mode first
	ok := p.autoplanValidator.InstrumentedIsValid(ctx, p.logger, event.Pull.BaseRepo, event.Pull.HeadRepo, event.Pull, event.User)
	if ok {
		err := p.workerProxy.Handle(ctx, request, event)
		if err != nil {
			return errors.Wrap(err, "handling autoplan")
		}
	} else {
		p.logger.WarnContext(ctx, "request isn't valid and will not be proxied to sns")
	}

	// TODO: remove allocator (only used for initial testing of PR workflow, not for rollout)
	allocate, err := p.allocator.ShouldAllocate(feature.PRMode, feature.FeatureContext{RepoName: event.Pull.HeadRepo.FullName})
	if err != nil {
		return errors.Wrap(err, "allocating PR mode")
	}
	if !allocate {
		return nil
	}
	prOptions := pr.Options{
		Number:            event.Pull.Num,
		Revision:          event.Pull.HeadCommit,
		Repo:              event.Pull.HeadRepo,
		InstallationToken: event.InstallationToken,
		Branch:            event.Pull.HeadBranch,
	}
	// Signal temporal worker
	return p.revisionHandler.Handle(ctx, prOptions)
}

func NewSNSWorkerProxy(
	snsWriter Writer,
	logger logging.Logger,
) *PullSNSWorkerProxy {
	return &PullSNSWorkerProxy{
		snsWriter: snsWriter,
		logger:    logger,
	}
}

type PullSNSWorkerProxy struct {
	snsWriter Writer
	logger    logging.Logger
}

func (p *PullSNSWorkerProxy) Handle(ctx context.Context, request *http.BufferedRequest, event PullRequest) error {
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
