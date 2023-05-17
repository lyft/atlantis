package event

// TODO: delete when legacy mode is deprecated
import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/http"
	"github.com/runatlantis/atlantis/server/logging"
)

type Writer interface {
	WriteWithContext(ctx context.Context, payload []byte) error
}

type PullSNSWorkerProxy struct {
	snsWriter Writer
	logger    logging.Logger
}

func NewSNSWorkerProxy(snsWriter Writer, logger logging.Logger) *PullSNSWorkerProxy {
	return &PullSNSWorkerProxy{
		snsWriter: snsWriter,
		logger:    logger,
	}
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
