package command

import (
	"context"
	"fmt"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_job_closer.go JobCloser

// Job Closer closes a job by marking op complete and clearing up buffers if logs are successfully persisted
type JobCloser interface {
	CloseJob(jobID string, repo models.Repo)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_job_url_generator.go ProjectJobURLGenerator

// JobURLGenerator generates urls to view project's progress.
type JobURLGenerator interface {
	GenerateProjectJobURL(p ProjectContext) (string, error)
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_project_status_updater.go ProjectStatusUpdater

type ProjectCommitStatusUpdater interface {
	// UpdateProject sets the commit status for the project represented by
	// ctx.
	UpdateProject(ctx context.Context, projectCtx ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string, statusId string) (string, error)
}

type StatusUpdater interface {
	UpdateProjectStatus(ctx ProjectContext, status models.CommitStatus) (string, error)
}

type ProjectStatusUpdater struct {
	ProjectJobURLGenerator     JobURLGenerator
	JobCloser                  JobCloser
	FeatureAllocator           feature.Allocator
	ProjectCommitStatusUpdater ProjectCommitStatusUpdater
}

func (p ProjectStatusUpdater) UpdateProjectStatus(ctx ProjectContext, status models.CommitStatus) (string, error) {
	url, err := p.ProjectJobURLGenerator.GenerateProjectJobURL(ctx)
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("updating project PR status %v", err))
	}
	statusId, err := p.ProjectCommitStatusUpdater.UpdateProject(context.TODO(), ctx, ctx.CommandName, status, url, ctx.StatusId)

	// Close the Job if the operation is complete
	if status == models.SuccessCommitStatus || status == models.FailedCommitStatus {
		p.JobCloser.CloseJob(ctx.JobID, ctx.BaseRepo)
	}
	return statusId, err
}

func (c *ProjectStatusUpdater) isChecksEnabled(ctx ProjectContext, repo models.Repo, pull models.PullRequest) bool {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         repo.FullName,
		PullCreationTime: pull.CreatedAt,
	})
	if err != nil {
		ctx.Log.ErrorContext(ctx.RequestCtx, fmt.Sprintf("unable to allocate for feature: %s, error: %s", feature.GithubChecks, err))
		return false
	}

	return shouldAllocate
}
