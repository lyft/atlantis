package activities

import (
	"context"

	"github.com/google/go-github/v45/github"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/temporalworker/job"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type githubActivities struct {
	ClientCreator   githubapp.ClientCreator
	JobURLGenerator job.UrlGenerator
}

type CreateCheckRunRequest struct {
	Title      string
	Sha        string
	Repo       internal.Repo
	State      internal.CheckRunState
	Conclusion internal.CheckRunConclusion
	JobID      string
}

type UpdateCheckRunRequest struct {
	Title      string
	State      internal.CheckRunState
	Conclusion internal.CheckRunConclusion
	Repo       internal.Repo
	ID         int64
	JobID      string
}

type CreateCheckRunResponse struct {
	ID int64
}
type UpdateCheckRunResponse struct {
	ID int64
}

func (a *githubActivities) UpdateCheckRun(ctx context.Context, request UpdateCheckRunRequest) (UpdateCheckRunResponse, error) {

	// TODO: Use this job id to generate checkrun summary with link to job URL
	// Job ID should never be empty since it's generated in the worker.
	// So let's fail the activity if there's any other error
	_, err := a.JobURLGenerator.GenerateJobURL(request.JobID)
	if err != nil {
		return UpdateCheckRunResponse{}, errors.Wrapf(err, "error generating job id")
	}

	output := github.CheckRunOutput{
		Title:   &request.Title,
		Text:    github.String("this is test"),
		Summary: github.String("this is also a test"),
	}

	opts := github.UpdateCheckRunOptions{
		Name:   request.Title,
		Status: github.String(string(request.State)),
		Output: &output,
	}

	// Conclusion is required if status is Completed
	if request.State == internal.CheckRunComplete {
		opts.Conclusion = github.String(string(request.Conclusion))
	}

	client, err := a.ClientCreator.NewInstallationClient(request.Repo.Credentials.InstallationToken)

	if err != nil {
		return UpdateCheckRunResponse{}, errors.Wrap(err, "creating installation client")
	}

	run, _, err := client.Checks.UpdateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, request.ID, opts)

	if err != nil {
		return UpdateCheckRunResponse{}, errors.Wrap(err, "creating check run")
	}

	return UpdateCheckRunResponse{
		ID: run.GetID(),
	}, nil
}

func (a *githubActivities) CreateCheckRun(ctx context.Context, request CreateCheckRunRequest) (CreateCheckRunResponse, error) {
	// TODO: Use this job id to generate checkrun summary with link to job URL
	// Job ID should never be empty since it's generated in the worker.
	// So let's fail the activity if there's any other error
	_, err := a.JobURLGenerator.GenerateJobURL(request.JobID)
	if err != nil {
		return CreateCheckRunResponse{}, errors.Wrapf(err, "error generating job id")
	}

	output := github.CheckRunOutput{
		Title:   &request.Title,
		Text:    github.String("this is test"),
		Summary: github.String("this is also a test"),
	}

	opts := github.CreateCheckRunOptions{
		Name:    request.Title,
		HeadSHA: request.Sha,
		Status:  github.String("queued"),
		Output:  &output,
	}

	var state internal.CheckRunState
	if request.State == internal.CheckRunState("") {
		state = internal.CheckRunQueued
	} else {
		state = request.State
	}

	opts.Status = github.String(string(state))

	// Conclusion is required if status is Completed
	if state == internal.CheckRunComplete {
		opts.Conclusion = github.String(string(request.Conclusion))
	}

	client, err := a.ClientCreator.NewInstallationClient(request.Repo.Credentials.InstallationToken)

	if err != nil {
		return CreateCheckRunResponse{}, errors.Wrap(err, "creating installation client")
	}

	run, _, err := client.Checks.CreateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, opts)

	if err != nil {
		return CreateCheckRunResponse{}, errors.Wrap(err, "creating check run")
	}

	return CreateCheckRunResponse{
		ID: run.GetID(),
	}, nil
}

type GithubRepoCloneRequest struct {
	Repo           internal.Repo
	Revision       string
	DestinationDir string
}

func (a *githubActivities) GithubRepoClone(ctx context.Context, request GithubRepoCloneRequest) error {
	return nil
}
