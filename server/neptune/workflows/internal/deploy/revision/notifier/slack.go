package notifier

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/revision/queue"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/deploy/lock"
	"github.com/slack-go/slack"
	"go.temporal.io/sdk/workflow"
)

type slackActivities interface {
	MessageChannel(ctx context.Context, request activities.MessageChannelRequest) (activities.MessageChannelResponse, error)
}

type Slack struct {
	DeployQueue *queue.Deploy
	Activities  slackActivities
}

func (s *Slack) Notify(ctx workflow.Context) error {
	state := s.DeployQueue.GetLockState()

	if state.Status == lock.UnlockedStatus {
		return nil
	}

	infos := s.DeployQueue.GetOrderedMergedItems()

	// if we have no merged items there is nowhere to unlock at the time of writing
	if len(infos) == 0 {
		return nil
	}

	// probably not great to get these from here but this
	// doesn't really change through the lifecycle of this workflow.
	revision := infos[0].Commit.Revision
	repo := infos[0].Repo.GetFullName()
	root := infos[0].Root.Name
	slackConfig := infos[0].Notifications.Slack

	if len(slackConfig.ChannelID) == 0 {
		return nil
	}

	err := workflow.ExecuteActivity(ctx, s.Activities.MessageChannel, activities.MessageChannelRequest{
		ChannelID: slackConfig.ChannelID,
		Message:   fmt.Sprintf("Deploys are locked for *%s* in *%s*.  Please navigate to the revision's check run to unlock the root", root, repo),
		Attachments: []slack.Attachment{
			{
				Title: " ",       // purposely empty
				Color: "#36a64f", // green
				Actions: []slack.AttachmentAction{
					{
						Type: "button",
						Text: "Open Revision",
						URL:  github.BuildRevisionURLMarkdown(repo, revision),
					},
				},
			},
		},
	}).Get(ctx, nil)

	if err != nil {
		return errors.Wrap(err, "messaging channel on lock status")
	}

	return nil
}
