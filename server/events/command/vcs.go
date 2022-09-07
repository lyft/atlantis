package command

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
)

const InfraDocsOpSummaryURL = "https://infradocs.lyft.net/deploy/infrastructure-as-code/intro.html#understanding-operation-summary"

// VCSStatusUpdater updates the status of a commit with the VCS host. We set
// the status to signify whether the plan/apply succeeds.
type VCSStatusUpdater struct {
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *VCSStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, statusId string) (string, error) {
	src := d.TitleBuilder.Build(cmdName.String())
	descrip := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))

	request := types.UpdateStatusRequest{
		Repo:        repo,
		PullNum:     pull.Num,
		Ref:         pull.HeadCommit,
		StatusName:  src,
		State:       status,
		Description: descrip,

		// Pass in a link to infra docs for aggregate operation checkruns to avoid routing users to a broken link
		DetailsURL:       InfraDocsOpSummaryURL,
		PullCreationTime: pull.CreatedAt,
		StatusId:         statusId,
		CommandName:      titleString(cmdName),
	}
	return d.Client.UpdateStatus(ctx, request)
}

func (d *VCSStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusId string) (string, error) {
	src := d.TitleBuilder.Build(cmdName.String())
	cmdVerb := "unknown"

	switch cmdName {
	case Plan:
		cmdVerb = "planned"
	case PolicyCheck:
		cmdVerb = "policies checked"
	case Apply:
		cmdVerb = "applied"
	}

	request := types.UpdateStatusRequest{
		Repo:        repo,
		PullNum:     pull.Num,
		Ref:         pull.HeadCommit,
		StatusName:  src,
		State:       status,
		Description: fmt.Sprintf("%d/%d projects %s successfully.", numSuccess, numTotal, cmdVerb),

		// Pass in a link to infra docs for aggregate operation checkruns to avoid routing users to a broken link
		DetailsURL:       InfraDocsOpSummaryURL,
		PullCreationTime: pull.CreatedAt,
		StatusId:         statusId,
		CommandName:      titleString(cmdName),

		// Additional fields for github checks rendering
		NumSuccess: strconv.FormatInt(int64(numSuccess), 10),
		NumTotal:   strconv.FormatInt(int64(numTotal), 10),
	}

	return d.Client.UpdateStatus(ctx, request)
}

func (d *VCSStatusUpdater) UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string, statusId string) (string, error) {
	projectID := projectCtx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", projectCtx.RepoRelDir, projectCtx.Workspace)
	}
	statusName := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	description := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))
	request := types.UpdateStatusRequest{
		Repo:             projectCtx.BaseRepo,
		PullNum:          projectCtx.Pull.Num,
		Ref:              projectCtx.Pull.HeadCommit,
		StatusName:       statusName,
		State:            status,
		Description:      description,
		DetailsURL:       url,
		PullCreationTime: projectCtx.Pull.CreatedAt,
		StatusId:         statusId,

		CommandName: titleString(cmdName),
		Project:     projectCtx.ProjectName,
		Workspace:   projectCtx.Workspace,
		Directory:   projectCtx.RepoRelDir,
	}

	return d.Client.UpdateStatus(ctx, request)
}

func (d *VCSStatusUpdater) statusDescription(status models.CommitStatus) string {
	var description string
	switch status {
	case models.PendingCommitStatus:
		description = "in progress..."
	case models.FailedCommitStatus:
		description = "failed."
	case models.SuccessCommitStatus:
		description = "succeeded."
	}

	return description
}

func titleString(cmdName fmt.Stringer) string {
	return strings.Title(strings.ReplaceAll(strings.ToLower(cmdName.String()), "_", " "))
}
