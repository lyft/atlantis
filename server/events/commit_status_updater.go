// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events

import (
	"fmt"
	"strings"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_commit_status_updater.go CommitStatusUpdater

// CommitStatusUpdater updates the status of a commit with the VCS host. We set
// the status to signify whether the plan/apply succeeds.
type CommitStatusUpdater interface {
	// UpdateCombined updates the combined status of the head commit of pull.
	// A combined status represents all the projects modified in the pull.
	UpdateCombined(repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName command.Name) error
	// UpdateCombinedCount updates the combined status to reflect the
	// numSuccess out of numTotal.
	UpdateCombinedCount(repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName command.Name, numSuccess int, numTotal int) error
	// UpdateProject sets the commit status for the project represented by
	// ctx.
	UpdateProject(ctx command.ProjectContext, cmdName command.Name, status models.CommitStatus, url string) error
}

type FeatureAwareCommitStatusUpdater struct {
	CommitStatusUpdater
	Client           vcs.Client
	FeatureAllocator feature.Allocator
}

func (f *FeatureAwareCommitStatusUpdater) UpdateProject(ctx command.ProjectContext, cmdName command.Name, status models.CommitStatus, url string) error {
	githubChecks, err := f.FeatureAllocator.ShouldAllocate(feature.GitHubChecks, ctx.HeadRepo.FullName)
	if err != nil {
		githubChecks = false
	}
	if githubChecks {
		if status == models.PendingCommitStatus {
			// This is a new commit or comment command so we create a new Check Run.
			checkId, err := f.Client.CreateCheckRun(ctx.BaseRepo, ctx.Pull, status, cmdName, url)
			if err != nil {
				return err
			}
			ctx.CheckID = checkId
		}
		return f.Client.UpdateCheckRun(ctx.BaseRepo, ctx.Pull, ctx.CheckID, status, cmdName, url, "")
	} else {
		return f.CommitStatusUpdater.UpdateProject(ctx, cmdName, status, url)
	}
}

// DefaultCommitStatusUpdater implements CommitStatusUpdater.
type DefaultCommitStatusUpdater struct {
	Client       vcs.Client
	TitleBuilder vcs.StatusTitleBuilder
}

func (d *DefaultCommitStatusUpdater) UpdateCombined(repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName command.Name) error {
	// add new implementation that calls into default implementation for status.
	src := d.TitleBuilder.Build(cmdName.String())
	descrip := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))
	return d.Client.UpdateStatus(repo, pull, status, src, descrip, "")
}

func (d *DefaultCommitStatusUpdater) UpdateCombinedCount(repo models.Repo, pull models.PullRequest, status models.CommitStatus, cmdName command.Name, numSuccess int, numTotal int) error {
	src := d.TitleBuilder.Build(cmdName.String())
	cmdVerb := "unknown"

	switch cmdName {
	case command.Plan:
		cmdVerb = "planned"
	case command.PolicyCheck:
		cmdVerb = "policies checked"
	case command.Apply:
		cmdVerb = "applied"
	}

	return d.Client.UpdateStatus(repo, pull, status, src, fmt.Sprintf("%d/%d projects %s successfully.", numSuccess, numTotal, cmdVerb), "")
}

func (d *DefaultCommitStatusUpdater) UpdateProject(ctx command.ProjectContext, cmdName command.Name, status models.CommitStatus, url string) error {
	projectID := ctx.ProjectName
	if projectID == "" {
		projectID = fmt.Sprintf("%s/%s", ctx.RepoRelDir, ctx.Workspace)
	}
	src := d.TitleBuilder.Build(cmdName.String(), vcs.StatusTitleOptions{
		ProjectName: projectID,
	})

	descrip := fmt.Sprintf("%s %s", strings.Title(cmdName.String()), d.statusDescription(status))
	return d.Client.UpdateStatus(ctx.BaseRepo, ctx.Pull, status, src, descrip, url)
}

func (d *DefaultCommitStatusUpdater) statusDescription(status models.CommitStatus) string {
	var descripWords string
	switch status {
	case models.PendingCommitStatus:
		descripWords = "in progress..."
	case models.FailedCommitStatus:
		descripWords = "failed."
	case models.SuccessCommitStatus:
		descripWords = "succeeded."
	}

	return descripWords
}
