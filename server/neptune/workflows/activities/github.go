package activities

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"

	key "github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/sdk/activity"

	"github.com/google/go-github/v45/github"
	"github.com/hashicorp/go-getter"
	"github.com/pkg/errors"
	internal "github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

var HashiGetter = func(ctx context.Context, dst, src string) error {
	return getter.Get(dst, src, getter.WithContext(ctx))
}

// wraps hashicorp's go getter to allow for testing
type gogetter func(ctx context.Context, dst, src string) error

type githubClient interface { //nolint:interfacebloat
	CreateCheckRun(ctx context.Context, owner, repo string, opts github.CreateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	UpdateCheckRun(ctx context.Context, owner, repo string, checkRunID int64, opts github.UpdateCheckRunOptions) (*github.CheckRun, *github.Response, error)
	GetArchiveLink(ctx context.Context, owner, repo string, archiveformat github.ArchiveFormat, opts *github.RepositoryContentGetOptions, followRedirects bool) (*url.URL, *github.Response, error)
	CompareCommits(ctx context.Context, owner, repo string, base, head string, opts *github.ListOptions) (*github.CommitsComparison, *github.Response, error)
	ListReviews(ctx context.Context, owner string, repo string, number int) ([]*github.PullRequestReview, error)
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
	ListCommits(ctx context.Context, owner string, repo string, number int) ([]*github.RepositoryCommit, error)
	DismissReview(ctx context.Context, owner, repo string, number int, reviewID int64, review *github.PullRequestReviewDismissalRequest) (*github.PullRequestReview, *github.Response, error)
	ListTeamMembers(ctx context.Context, org string, teamSlug string) ([]*github.User, error)
	CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error)
}

type DiffDirection string

const (
	DirectionAhead     DiffDirection = "ahead"
	DirectionBehind    DiffDirection = "behind"
	DirectionIdentical DiffDirection = "identical"
	DirectionDiverged  DiffDirection = "diverged"
)

const (
	deploymentsDirName = "deployments"
)

type githubActivities struct {
	Client      githubClient
	DataDir     string
	LinkBuilder LinkBuilder
	Getter      gogetter
	Allocator   feature.Allocator
}

type CreateCheckRunRequest struct {
	Title      string
	Sha        string
	Repo       internal.Repo
	State      internal.CheckRunState
	Actions    []internal.CheckRunAction
	Summary    string
	ExternalID string
	Mode       terraform.WorkflowMode
}

type UpdateCheckRunRequest struct {
	Title      string
	State      internal.CheckRunState
	Actions    []internal.CheckRunAction
	Repo       internal.Repo
	ID         int64
	Summary    string
	ExternalID string
	Mode       terraform.WorkflowMode
}

type CreateCheckRunResponse struct {
	ID     int64
	Status string
}

type UpdateCheckRunResponse struct {
	ID     int64
	Status string
}

func (a *githubActivities) GithubUpdateCheckRun(ctx context.Context, request UpdateCheckRunRequest) (UpdateCheckRunResponse, error) {
	output := github.CheckRunOutput{
		Title:   &request.Title,
		Text:    &request.Title,
		Summary: &request.Summary,
	}

	state, conclusion := getCheckStateAndConclusion(request.State)

	opts := github.UpdateCheckRunOptions{
		Name:       request.Title,
		Status:     github.String(state),
		Output:     &output,
		ExternalID: &request.ExternalID,
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
		ctx,
		request.Repo.Owner, request.Repo.Name, request.ID, opts,
	)

	if err != nil {
		return UpdateCheckRunResponse{}, errors.Wrap(err, "creating check run")
	}

	return UpdateCheckRunResponse{
		ID:     run.GetID(),
		Status: run.GetStatus(),
	}, nil
}

func (a *githubActivities) GithubCreateCheckRun(ctx context.Context, request CreateCheckRunRequest) (CreateCheckRunResponse, error) {
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

	// update with any actions
	if len(request.Actions) != 0 {
		var actions []*github.CheckRunAction

		for _, a := range request.Actions {
			actions = append(actions, a.ToGithubAction())
		}

		opts.Actions = actions
	}

	if conclusion != "" {
		opts.Conclusion = &conclusion
	}

	run, _, err := a.Client.CreateCheckRun(
		ctx,
		request.Repo.Owner, request.Repo.Name, opts,
	)

	if err != nil {
		return CreateCheckRunResponse{}, errors.Wrap(err, "creating check run")
	}

	return CreateCheckRunResponse{
		ID:     run.GetID(),
		Status: run.GetStatus(),
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
	case internal.CheckRunTimeout:
		state = "completed"
		conclusion = "timed_out"
	case internal.CheckRunActionRequired:
		state = "completed"
		conclusion = "action_required"
	case internal.CheckRunSkipped:
		state = "completed"
		conclusion = "skipped"
	case internal.CheckRunCancelled:
		state = "completed"
		conclusion = "cancelled"
	default:
		state = string(internalState)
	}

	return state, conclusion
}

type FetchRootRequest struct {
	Repo         internal.Repo
	Root         terraform.Root
	DeploymentID string
	Revision     string
	WorkflowMode terraform.WorkflowMode
}

type FetchRootResponse struct {
	LocalRoot *terraform.LocalRoot

	// if we do more with this, we can consider moving generation into it's own activity
	DeployDirectory string
}

// FetchRoot fetches a link to the archive URL using the GH client, processes that URL into a download URL that the
// go-getter library can use, and then go-getter to download/extract files/subdirs within the root path to the destinationPath.
func (a *githubActivities) GithubFetchRoot(ctx context.Context, request FetchRootRequest) (FetchRootResponse, error) {
	cancel := temporal.StartHeartbeat(ctx, temporal.HeartbeatTimeout)
	defer cancel()

	deployBasePath := filepath.Join(a.DataDir, deploymentsDirName, request.DeploymentID)
	// if we are in Adhoc mode, we can use a simple file path without a UUID since there will only be one repository
	if request.WorkflowMode == terraform.Adhoc {
		deployBasePath = filepath.Join(a.DataDir)
	}
	repositoryPath := filepath.Join(deployBasePath, "repo")
	opts := &github.RepositoryContentGetOptions{
		Ref: request.Revision,
	}
	// note: this link exists for 5 minutes when fetching a private repository archive
	archiveLink, resp, err := a.Client.GetArchiveLink(ctx, request.Repo.Owner, request.Repo.Name, github.Zipball, opts, true)
	if err != nil {
		return FetchRootResponse{}, errors.Wrap(err, "getting repo archive link")
	}
	// GH responds with a 302 + redirect link to where the archive exists
	if resp.StatusCode != http.StatusFound {
		return FetchRootResponse{}, errors.Errorf("getting repo archive link returns non-302 status %d", resp.StatusCode)
	}
	downloadLink := a.LinkBuilder.BuildDownloadLinkFromArchive(archiveLink, request.Repo, request.Revision)
	err = a.Getter(ctx, repositoryPath, downloadLink)
	if err != nil {
		return FetchRootResponse{}, errors.Wrap(err, "fetching and extracting zip")
	}
	rootPath := filepath.Join(repositoryPath, request.Root.Path)

	// let's drop a symlink to the root in the deploy base path to make navigation easier
	rootSymlink := filepath.Join(deployBasePath, "root")
	err = os.Symlink(rootPath, rootSymlink)
	if err != nil {
		activity.GetLogger(ctx).Warn("unable to symlink to terraform root", key.ErrKey, err)
	}

	localRoot := terraform.BuildLocalRoot(request.Root, request.Repo, rootPath)
	return FetchRootResponse{
		LocalRoot:       localRoot,
		DeployDirectory: deployBasePath,
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

func (a *githubActivities) GithubCompareCommit(ctx context.Context, request CompareCommitRequest) (CompareCommitResponse, error) {
	comparison, resp, err := a.Client.CompareCommits(ctx, request.Repo.Owner, request.Repo.Name, request.LatestDeployedRevision, request.DeployRequestRevision, &github.ListOptions{})

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

type GetPullRequestStateRequest struct {
	Repo     internal.Repo
	PRNumber int
}

type GetPullRequestStateResponse struct {
	State string
}

func (a *githubActivities) GithubGetPullRequestState(ctx context.Context, request GetPullRequestStateRequest) (GetPullRequestStateResponse, error) {
	resp, _, err := a.Client.GetPullRequest(
		ctx,
		request.Repo.Owner,
		request.Repo.Name,
		request.PRNumber,
	)
	if err != nil {
		return GetPullRequestStateResponse{}, errors.Wrap(err, "fetching PR status")
	}
	return GetPullRequestStateResponse{
		State: resp.GetState(),
	}, nil
}

type ListPRReviewsRequest struct {
	Repo     internal.Repo
	PRNumber int
}

type ListPRReviewsResponse struct {
	Reviews []*github.PullRequestReview
}

func (a *githubActivities) GithubListPRReviews(ctx context.Context, request ListPRReviewsRequest) (ListPRReviewsResponse, error) {
	reviews, err := a.Client.ListReviews(
		ctx,
		request.Repo.Owner,
		request.Repo.Name,
		request.PRNumber,
	)
	if err != nil {
		return ListPRReviewsResponse{}, errors.Wrap(err, "listing approvals from pr")
	}
	return ListPRReviewsResponse{
		Reviews: reviews,
	}, nil
}

type ListPRCommitsRequest struct {
	Repo     internal.Repo
	PRNumber int
}

type ListPRCommitsResponse struct {
	Commits []*github.RepositoryCommit
}

func (a *githubActivities) GithubListPRCommits(ctx context.Context, request ListPRCommitsRequest) (ListPRCommitsResponse, error) {
	commits, err := a.Client.ListCommits(
		ctx,
		request.Repo.Owner,
		request.Repo.Name,
		request.PRNumber,
	)
	if err != nil {
		return ListPRCommitsResponse{}, errors.Wrap(err, "listing commits from pr")
	}
	return ListPRCommitsResponse{
		Commits: commits,
	}, nil
}

type DismissRequest struct {
	Repo          internal.Repo
	PRNumber      int
	ReviewID      int64
	DismissReason string
}

type DismissResponse struct{}

func (a *githubActivities) GithubDismiss(ctx context.Context, request DismissRequest) (DismissResponse, error) {
	dismissRequest := &github.PullRequestReviewDismissalRequest{
		Message: github.String(request.DismissReason),
	}
	_, _, err := a.Client.DismissReview(
		ctx,
		request.Repo.Owner,
		request.Repo.Name,
		request.PRNumber,
		request.ReviewID,
		dismissRequest,
	)
	if err != nil {
		return DismissResponse{}, errors.Wrap(err, "dismissing pr review")
	}
	return DismissResponse{}, nil
}

type ListTeamMembersRequest struct {
	Repo     internal.Repo
	Org      string
	TeamSlug string
}

type ListTeamMembersResponse struct {
	Members []string
}

func (a *githubActivities) GithubListTeamMembers(ctx context.Context, request ListTeamMembersRequest) (ListTeamMembersResponse, error) {
	users, err := a.Client.ListTeamMembers(
		ctx,
		request.Org,
		request.TeamSlug,
	)
	if err != nil {
		return ListTeamMembersResponse{}, errors.Wrap(err, "listing team members")
	}
	var members []string
	for _, user := range users {
		members = append(members, user.GetLogin())
	}
	return ListTeamMembersResponse{
		Members: members,
	}, nil
}

type CreateCommentRequest struct {
	Repo        internal.Repo
	PRNumber    int
	CommentBody string
}

type CreateCommentResponse struct{}

func (a *githubActivities) GithubCreateComment(ctx context.Context, request CreateCommentRequest) (CreateCommentResponse, error) {
	comment := &github.IssueComment{
		Body: github.String(request.CommentBody),
	}
	_, _, err := a.Client.CreateComment(
		ctx,
		request.Repo.Owner,
		request.Repo.Name,
		request.PRNumber,
		comment,
	)
	if err != nil {
		return CreateCommentResponse{}, errors.Wrap(err, "creating comment on PR")
	}
	return CreateCommentResponse{}, nil
}
