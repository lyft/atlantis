package converter

import (
	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
)

type ChecksEvent struct{}

func (p ChecksEvent) Convert(e *github.CheckRunEvent) (event.CheckRun, error) {
	var action event.CheckRunAction
	switch e.GetAction() {
	case "requested_action":
		action = event.RequestedActionChecksAction{
			Identifier: e.GetRequestedAction().Identifier,
		}
	default:
		action = event.WrappedCheckRunAction(e.GetAction())
	}

	return event.CheckRun{
		Action:     action,
		ExternalID: e.CheckRun.GetExternalID(),
	}, nil

}
