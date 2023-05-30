package activities

import (
	"context"

	"github.com/slack-go/slack"
)

type slackActivities struct {
	Client slack.Client
}

type MessageChannelRequest struct {
	ChannelID   string
	Message     string
	Attachments []slack.Attachment
}

type MessageChannelResponse struct {
	MessageID string
}

func (a *slackActivities) MessageChannel(ctx context.Context, request MessageChannelRequest) (MessageChannelResponse, error) {
	_, ts, err := a.Client.PostMessageContext(
		ctx, request.ChannelID,
		slack.MsgOptionText(request.Message, true),
		slack.MsgOptionPostMessageParameters(slack.PostMessageParameters{Markdown: true}),
		slack.MsgOptionAttachments(request.Attachments...),
	)
	if err != nil {
		return MessageChannelResponse{}, err
	}

	return MessageChannelResponse{MessageID: ts}, nil
}
