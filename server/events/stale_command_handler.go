package events

import (
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/uber-go/tally"
)

type StaleCommandHandler struct {
	StaleStatsScope tally.Scope
}

func (s *StaleCommandHandler) CommandIsStale(ctx *models.CommandContext) bool {
	status := ctx.PullStatus
	if status != nil && status.UpdatedAt > ctx.TriggerTimestamp.Unix() {
		s.StaleStatsScope.Counter("dropped_commands").Inc(1)
		return true
	}
	return false
}
