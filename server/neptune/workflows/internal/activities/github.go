package activities

import (
	"context"
	"fmt"
	"github.com/google/go-github/v45/github"
	"github.com/hashicorp/go-getter"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"net/url"
	"path"
	"strings"
)

type githubActivities struct {
	ClientCreator githubapp.ClientCreator
}

type CreateCheckRunRequest struct {
	Title      string
	Sha        string
	Repo       internal.Repo
	State      internal.CheckRunState
	Conclusion internal.CheckRunConclusion
}

type UpdateCheckRunRequest struct {
	Title      string
	State      internal.CheckRunState
	Conclusion internal.CheckRunConclusion
	Repo       internal.Repo
	ID         int64
}

type CreateCheckRunResponse struct {
	ID int64
}
type UpdateCheckRunResponse struct {
	ID int64
}

func (a *githubActivities) UpdateCheckRun(ctx context.Context, request UpdateCheckRunRequest) (UpdateCheckRunResponse, error) {
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

type CloneRepoRequest struct {
	Repo            internal.Repo
	RootPath        string
	DestinationPath string
}

// CloneRepoResponse empty for now,
// but keeping it makes it easier to change in the future.
type CloneRepoResponse struct{}

func (a *githubActivities) CloneRepo(ctx context.Context, request CloneRepoRequest) (CloneRepoResponse, error) {
	// Fetch link for zip
	opts := &github.RepositoryContentGetOptions{
		Ref: request.Repo.Ref,
	}
	client, err := a.ClientCreator.NewInstallationClient(request.Repo.Credentials.InstallationToken)
	if err != nil {
		return CloneRepoResponse{}, errors.Wrap(err, "creating installation client")
	}
	url, resp, err := client.Repositories.GetArchiveLink(ctx, request.Repo.Owner, request.Repo.Name, github.Zipball, opts, true)
	if err != nil {
		return CloneRepoResponse{}, errors.Wrap(err, "repository get contents")
	}
	if resp.StatusCode != 302 {
		return CloneRepoResponse{}, errors.New("not found status returned on download contents")
	}
	// Fetch archive, unarchive contents into destination path, and remove out any unnecessary files
	srcURL := buildSrcURL(url, request)
	err = getter.GetAny(request.DestinationPath, srcURL, getter.WithContext(ctx))
	if err != nil {
		return CloneRepoResponse{}, errors.Wrapf(err, "fetching and unarchiving zip")
	}
	return CloneRepoResponse{}, nil
}

// buildSrcURL modifies fetched archive URL to add needed query/path modifications for the go-getter pkg to handle
// un-archiving entire repo and fetching files only defined within rootPath
func buildSrcURL(url *url.URL, request CloneRepoRequest) string {
	// Append zip query param to trigger go-getter pkg to un-archive contents
	queryParams := "archive=zip"
	token := url.Query().Get("token")
	if token != "" {
		queryParams += fmt.Sprintf("&token=%s", token)
	}
	url.RawQuery = queryParams
	// Subdirectory will exist under the name of the parent unarchived directory
	archiveName := strings.Join([]string{request.Repo.Owner, request.Repo.Name, request.Repo.Ref}, "-")
	subDirPath := path.Join(archiveName, request.RootPath)
	url.Path = fmt.Sprintf("%s//%s", url.Path, subDirPath)
	return url.String()
}
