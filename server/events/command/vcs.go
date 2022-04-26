package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
)

type ChecksEnabledVCSStatusUpdater struct {
	VCSStatusUpdater
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *ChecksEnabledVCSStatusUpdater) CreateProjectStatus(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus) (string, error) {
	projectID := projectCtx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", projectCtx.RepoRelDir, projectCtx.Workspace)
	}
	statusName := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	request := types.CreateStatusRequest{
		Repo:       projectCtx.BaseRepo,
		PullNum:    projectCtx.Pull.Num,
		Ref:        projectCtx.Pull.HeadCommit,
		StatusName: statusName,
		State:      status,
	}

	return d.Client.CreateStatus(ctx, request)
}

func (d *ChecksEnabledVCSStatusUpdater) CreateCommandStatus(ctx context.Context, pull models.PullRequest, repo models.Repo, cmdName fmt.Stringer, status models.CommitStatus) (string, error) {
	request := types.CreateStatusRequest{
		Repo:       repo,
		PullNum:    pull.Num,
		Ref:        pull.HeadCommit,
		StatusName: cmdName.String(),
		State:      status,
	}

	return d.Client.CreateStatus(ctx, request)
}

// VCSStatusUpdater updates the status of a commit with the VCS host. We set
// the status to signify whether the plan/apply succeeds.
type VCSStatusUpdater struct {
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *VCSStatusUpdater) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, statusID string) error {
	src := d.TitleBuilder.Build(cmdName.String())
	descrip := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))

	request := types.UpdateStatusRequest{
		Repo:        repo,
		PullNum:     pull.Num,
		Ref:         pull.HeadCommit,
		StatusName:  src,
		State:       status,
		Description: descrip,
		DetailsURL:  "",
		StatusId:    statusID,
	}
	return d.Client.UpdateStatus(ctx, request)
}

func (d *VCSStatusUpdater) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) error {
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
		StatusId:    statusID,
	}

	return d.Client.UpdateStatus(ctx, request)
}

func (d *VCSStatusUpdater) UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string) error {
	projectID := projectCtx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", projectCtx.RepoRelDir, projectCtx.Workspace)
	}
	statusName := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	description := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))
	request := types.UpdateStatusRequest{
		Repo:        projectCtx.BaseRepo,
		PullNum:     projectCtx.Pull.Num,
		Ref:         projectCtx.Pull.HeadCommit,
		StatusName:  statusName,
		State:       status,
		Description: description,
		DetailsURL:  url,
		StatusId:    projectCtx.StatusID,
	}

	return d.Client.UpdateStatus(ctx, request)
}

// VCSStatusUpdater relays this call to UpdateProject since statusID is not generated
func (d *VCSStatusUpdater) CreateProjectStatus(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus) (string, error) {
	err := d.UpdateProject(ctx, projectCtx, cmdName, status, "")
	return "", err
}

// VCSStatusUpdater relays this call to UpdateCombined since statusID is not generated
func (d *VCSStatusUpdater) CreateCommandStatus(ctx context.Context, pull models.PullRequest, repo models.Repo, cmdName fmt.Stringer, status models.CommitStatus) (string, error) {
	err := d.UpdateCombined(ctx, repo, pull, status, cmdName, "")
	return "", err
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
