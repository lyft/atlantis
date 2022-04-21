package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
)

// GithubChecksStatusUpdater is used to update the github status checks
type ChecksStatusUpdater struct {
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *ChecksStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer) error {
	// No need to update combined status when using github checks
	return nil
}

func (d *ChecksStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, numSuccess int, numTotal int) error {
	// No need to update combined count status when using github checks
	return nil
}

func (d *ChecksStatusUpdater) UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string, output string) (string, error) {
	projectID := projectCtx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", projectCtx.RepoRelDir, projectCtx.Workspace)
	}
	statusName := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	description := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), statusDescription(status))
	request := types.UpdateStatusRequest{
		Repo:        projectCtx.BaseRepo,
		PullNum:     projectCtx.Pull.Num,
		Ref:         projectCtx.Pull.HeadCommit,
		StatusName:  statusName,
		State:       status,
		Description: description,
		DetailsURL:  url,
	}

	// Only update output when output string is provided
	if output != "" {
		request.Output = types.JobOutput{
			Title:   statusName,
			Summary: description,
			Text:    output,
		}
	}

	return d.Client.UpdateStatus(ctx, request, projectCtx.CheckRunId)
}

// VCSStatusUpdater updates the status of a commit with the VCS host. We set
// the status to signify whether the plan/apply succeeds.
type VCSStatusUpdater struct {
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *VCSStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer) error {
	src := d.TitleBuilder.Build(cmdName.String())
	descrip := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), statusDescription(status))

	request := types.UpdateStatusRequest{
		Repo:        repo,
		PullNum:     pull.Num,
		Ref:         pull.HeadCommit,
		StatusName:  src,
		State:       status,
		Description: descrip,
		DetailsURL:  "",
	}
	_, err := d.Client.UpdateStatus(ctx, request, "")
	return err
	return nil
}

func (d *VCSStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, numSuccess int, numTotal int) error {
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
		DetailsURL:  "",
	}

	_, err := d.Client.UpdateStatus(ctx, request, "")
	return err
	return nil
}

func (d *VCSStatusUpdater) UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string, _ string) (string, error) {
	projectID := projectCtx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", projectCtx.RepoRelDir, projectCtx.Workspace)
	}
	statusName := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	description := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), statusDescription(status))
	request := types.UpdateStatusRequest{
		Repo:        projectCtx.BaseRepo,
		PullNum:     projectCtx.Pull.Num,
		Ref:         projectCtx.Pull.HeadCommit,
		StatusName:  statusName,
		State:       status,
		Description: description,
		DetailsURL:  url,
	}

	return d.Client.UpdateStatus(ctx, request, projectCtx.CheckRunId)
}

func statusDescription(status models.CommitStatus) string {
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
