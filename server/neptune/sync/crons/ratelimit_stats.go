package crons

import (
	"context"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/uber-go/tally/v4"
	"net/http"
)

const (
	subScope  = "github.ratelimit"
	remaining = "remaining"
)

type RateLimitStatCollector struct {
	Scope          tally.Scope
	ClientCreator  githubapp.ClientCreator
	InstallationID int64
}

func NewRateLimitStats(scope tally.Scope, clientCreator githubapp.ClientCreator, installationID int64) *RateLimitStatCollector {
	return &RateLimitStatCollector{
		Scope:          scope.SubScope(subScope),
		ClientCreator:  clientCreator,
		InstallationID: installationID,
	}
}

func (r *RateLimitStatCollector) Run(ctx context.Context) error {
	installationClient, err := r.ClientCreator.NewInstallationClient(r.InstallationID)
	if err != nil {
		return errors.Wrap(err, "creating installation client")
	}

	rateLimits, resp, err := installationClient.RateLimits(ctx)
	if err != nil || resp.StatusCode != http.StatusOK {
		r.Scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return errors.Wrap(err, "fetching github ratelimit")
	}

	r.Scope.Gauge(remaining).Update(float64(rateLimits.GetCore().Remaining))
	return nil
}
