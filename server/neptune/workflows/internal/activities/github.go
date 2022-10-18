package activities

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
)

type ClientContext struct {
	InstallationToken int64
	context.Context
}

var HashiGetter = func(ctx context.Context, dst, src string) error {
	return getter.Get(dst, src, getter.WithContext(ctx))
}

// wraps hashicorp's go getter to allow for testing
type gogetter func(ctx context.Context, dst, src string) error

type githubClient interface {
	CreateCheckRun(ctx internal.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	UpdateCheckRun(ctx internal.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	GetArchiveLink(ctx internal.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, followRedirects bool) (*url.URL, *github.Response, error)
	CompareCommits(ctx internal.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error)
}

type DiffDirection string

const (
	DirectionAhead     DiffDirection = "ahead"
	DirectionBehind    DiffDirection = "behind"
	DirectionIdentical DiffDirection = "identical"
	DirectionDiverged  DiffDirection = "diverged"
)

const deploymentsDirName = "deployments"

type githubActivities struct {
	Client      githubClient
	DataDir     string
	LinkBuilder LinkBuilder
	Getter      gogetter
}

type CreateCheckRunRequest struct {
	Title      string
	Sha        string
	Repo       internal.Repo
	State      internal.CheckRunState
	Summary    string
	ExternalID string
}

type UpdateCheckRunRequest struct {
	Title   string
	State   internal.CheckRunState
	Actions []internal.CheckRunAction
	Repo    internal.Repo
	ID      int64
	Summary string
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
		Text:    &request.Title,
		Summary: &request.Summary,
	}

	state, conclusion := getCheckStateAndConclusion(request.State)

	opts := github.UpdateCheckRunOptions{
		Name:   request.Title,
		Status: github.String(state),
		Output: &output,
	}

	// update with any actions
	if len(request.Actions) != 0 {
		var actions []*github.CheckRunAction

		for _, a := range request.Actions {
			actions = append(actions, a.ToGithubAction())
		}

		opts.Actions = actions
	}

	if conclusion != "" {
		opts.Conclusion = github.String(conclusion)
	}

	run, _, err := a.Client.UpdateCheckRun(
		internal.ContextWithInstallationToken(ctx, request.Repo.Credentials.InstallationToken),
		request.Repo.Owner, request.Repo.Name, request.ID, opts,
	)

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
		Text:    &request.Title,
		Summary: &request.Summary,
	}

	state, conclusion := getCheckStateAndConclusion(request.State)

	opts := github.CreateCheckRunOptions{
		Name:       request.Title,
		HeadSHA:    request.Sha,
		Status:     &state,
		Output:     &output,
		ExternalID: &request.ExternalID,
	}

	if conclusion != "" {
		opts.Conclusion = &conclusion
	}

	run, _, err := a.Client.CreateCheckRun(
		internal.ContextWithInstallationToken(ctx, request.Repo.Credentials.InstallationToken),
		request.Repo.Owner, request.Repo.Name, opts,
	)

	if err != nil {
		return CreateCheckRunResponse{}, errors.Wrap(err, "creating check run")
	}

	return CreateCheckRunResponse{
		ID: run.GetID(),
	}, nil
}

func getCheckStateAndConclusion(internalState internal.CheckRunState) (string, string) {
	var state string
	var conclusion string
	// checks are weird in that success and failure are defined in the conclusion, and the state
	// is just marked as complete, let's just deal with that stuff here because it's not intuitive for
	// callers
	switch internalState {

	// default to queued if we have nothing
	case internal.CheckRunUnknown:
		state = string(internal.CheckRunQueued)
	case internal.CheckRunFailure:
		state = "completed"
		conclusion = "failure"
	case internal.CheckRunSuccess:
		state = "completed"
		conclusion = "success"
	default:
		state = string(internalState)
	}

	return state, conclusion
}

type FetchRootRequest struct {
	Repo         internal.Repo
	Root         root.Root
	DeploymentID string
	Revision     string
}

type FetchRootResponse struct {
	LocalRoot *root.LocalRoot
}

// FetchRoot fetches a link to the archive URL using the GH client, processes that URL into a download URL that the
// go-getter library can use, and then go-getter to download/extract files/subdirs within the root path to the destinationPath.
func (a *githubActivities) FetchRoot(ctx context.Context, request FetchRootRequest) (FetchRootResponse, error) {
	ctx, cancel := temporal.StartHeartbeat(ctx, 10*time.Second)
	defer cancel()
	ref, err := request.Repo.Ref.String()
	if err != nil {
		return FetchRootResponse{}, errors.Wrap(err, "processing request ref")
	}
	destinationPath := filepath.Join(a.DataDir, deploymentsDirName, request.DeploymentID)
	opts := &github.RepositoryContentGetOptions{
		Ref: ref,
	}
	// note: this link exists for 5 minutes when fetching a private repository archive
	archiveLink, resp, err := a.Client.GetArchiveLink(internal.ContextWithInstallationToken(ctx, request.Repo.Credentials.InstallationToken), request.Repo.Owner, request.Repo.Name, github.Zipball, opts, true)
	if err != nil {
		return FetchRootResponse{}, errors.Wrap(err, "getting repo archive link")
	}
	// GH responds with a 302 + redirect link to where the archive exists
	if resp.StatusCode != http.StatusFound {
		return FetchRootResponse{}, errors.Errorf("getting repo archive link returns non-302 status %d", resp.StatusCode)
	}
	downloadLink := a.LinkBuilder.BuildDownloadLinkFromArchive(archiveLink, request.Root, request.Repo, request.Revision)
	err = a.Getter(ctx, destinationPath, downloadLink)
	if err != nil {
		return FetchRootResponse{}, errors.Wrap(err, "fetching and extracting zip")
	}
	localRoot := root.BuildLocalRoot(request.Root, request.Repo, destinationPath)
	return FetchRootResponse{
		LocalRoot: localRoot,
	}, nil
}

type CompareCommitRequest struct {
	DeployRequestRevision  string
	LatestDeployedRevision string
	Repo                   internal.Repo
}

type CompareCommitResponse struct {
	CommitComparison DiffDirection
}

func (a *githubActivities) CompareCommit(ctx context.Context, request CompareCommitRequest) (CompareCommitResponse, error) {
	comparison, resp, err := a.Client.CompareCommits(internal.ContextWithInstallationToken(ctx, request.Repo.Credentials.InstallationToken), request.Repo.Owner, request.Repo.Name, request.LatestDeployedRevision, request.DeployRequestRevision, &github.ListOptions{})

	if err != nil {
		return CompareCommitResponse{}, errors.Wrap(err, "comparing commits")
	}

	if comparison.GetStatus() == "" || resp.StatusCode != http.StatusOK {
		return CompareCommitResponse{}, fmt.Errorf("invalid commit comparison status: %s, Status Code: %d", comparison.GetStatus(), resp.StatusCode)
	}

	return CompareCommitResponse{
		CommitComparison: DiffDirection(comparison.GetStatus()),
	}, nil
}
