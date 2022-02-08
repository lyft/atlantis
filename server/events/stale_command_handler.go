package events

import (
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/uber-go/tally"
)

type StaleCommandHandler struct {
	Counter tally.Counter
}

func (s *StaleCommandHandler) CommandIsStale(ctx models.ProjectCommandContext, fetcher PullStatusFetcher) bool {
	// We need to re-fetch the pull model within the lock
	status, err := fetcher.GetPullStatus(ctx.Pull)
	// Assume failed fetches are stale and have user retry command
	if err != nil {
		s.Counter.Inc(1)
		return true
	}
	if status != nil && status.LastEventTimestamp.After(ctx.EventTimestamp) {
		s.Counter.Inc(1)
		return true
	}
	return false
}
