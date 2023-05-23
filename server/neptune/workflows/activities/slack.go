package activities

import (
	"context"
)

type slackActivities struct {}

type MessageChannelRequest struct {
	ChannelID string
	Message   string
}

type MessageChannelResponse struct {
	MessageID string
}

func (a *slackActivities) MessageChannel(ctx context.Context, request MessageChannelRequest) (MessageChannelResponse, error) {
	return MessageChannelResponse{"123"}, nil
}
