package events

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/aws/sns"
	"github.com/uber-go/tally"
	"net/http"
	"time"
)

type GatewayCommandExecutor struct {
	SNSWriter     sns.Writer
	CommandRunner events.CommandRunner
	Scope         tally.Scope
	Logger        logging.SimpleLogging
}

func (g *GatewayCommandExecutor) ExecuteCommentCommand(request *http.Request, _ models.Repo, _ *models.Repo, _ *models.PullRequest, _ models.User, _ int, _ *events.CommentCommand, _ time.Time) HttpResponse {
	if err := g.SendToWorker(request); err != nil {
		g.Logger.With("err", err).Warn("failed to send comment request to Atlantis worker")
		return HttpResponse{
			body: err.Error(),
			err: HttpError{
				code: http.StatusBadRequest,
				err:  err,
			},
		}
	}
	return HttpResponse{
		body: "Processing...",
	}
}

func (g *GatewayCommandExecutor) ExecuteAutoplanCommand(request *http.Request, eventType models.PullRequestEventType, baseRepo models.Repo, headRepo models.Repo, pull models.PullRequest, user models.User, _ time.Time, _ logging.SimpleLogging) HttpResponse {
	var err error
	switch eventType {
	case models.OpenedPullEvent, models.UpdatedPullEvent:
		// If the pull request was opened or updated, we perform a pseudo-autoplan to determine if tf changes exist.
		// If they exist, then we will forward request to the worker.
		if hasTerraformChanges := g.CommandRunner.RunPseudoAutoplanCommand(baseRepo, headRepo, pull, user); hasTerraformChanges {
			if err = g.SendToWorker(request); err != nil {
				g.Logger.With("err", err, "eventType", eventType).Warn("failed to send autoplan request to Atlantis worker")
				return HttpResponse{
					body: err.Error(),
					err: HttpError{
						code: http.StatusBadRequest,
						err:  err,
					},
				}
			}
			return HttpResponse{
				body: "Processing...",
			}
		}
	case models.ClosedPullEvent:
		// If the pull request was closed, we route to worker to handle deleting locks.
		if err = g.SendToWorker(request); err != nil {
			g.Logger.With("err", err, "eventType", eventType).Warn("failed to send autoplan request to Atlantis worker")
			return HttpResponse{
				body: err.Error(),
				err: HttpError{
					code: http.StatusBadRequest,
					err:  err,
				},
			}
		}
	case models.OtherPullEvent:
		// Else we ignore the event.
		return HttpResponse{
			body: "Ignoring non-actionable pull request event",
		}
	}
	return HttpResponse{}
}

func (g *GatewayCommandExecutor) SendToWorker(r *http.Request) error {
	buffer := bytes.NewBuffer([]byte{})
	if err := r.Write(buffer); err != nil {
		g.Scope.SubScope("send").Counter("failure").Inc(1)
		return errors.Wrap(err, "marshalling gateway request to buffer")
	}
	if err := g.SNSWriter.Write(buffer.Bytes()); err != nil {
		g.Scope.SubScope("send").Counter("failure").Inc(1)
		return errors.Wrap(err, "writing gateway message to sns topic")
	}
	g.Scope.SubScope("send").Counter("success").Inc(1)
	return nil
}
