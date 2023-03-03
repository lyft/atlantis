package queue

import (
	"context"
	"time"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	"go.temporal.io/sdk/workflow"
)

// GH API Ratelimit resets every hour, 2 hours should be enough for the GH API call failures to autoresolve
const RebaseGithubScheduleToCloseTimeout = 2 * time.Hour
const RebaseTaskQueue = "rebase"

type rebaseActivities interface {
	GithubListOpenPRs(ctx context.Context, request activities.ListOpenPRsRequest) (activities.ListOpenPRsResponse, error)
	GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error)
	SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) (activities.SetPRRevisionResponse, error)
}

type PullRebaser struct {
	RebaseActivites rebaseActivities
}

// Called async in the deploy workflow after a deployment is complete
func (p *PullRebaser) RebaseOpenPRsForRoot(ctx workflow.Context, repo github.Repo, root terraform.Root, scope metrics.Scope) error {

	// custom tq for rebase activities to allow flow control and limit GH API calls per hour
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           RebaseTaskQueue,
		StartToCloseTimeout: 10 * time.Second,
	})

	// list open PRs
	var listOpenPRsResp activities.ListOpenPRsResponse
	err := workflow.ExecuteActivity(ctx, p.RebaseActivites.GithubListOpenPRs, activities.ListOpenPRsRequest{
		Repo: repo,
	}).Get(ctx, &listOpenPRsResp)
	if err != nil {
		return errors.Wrap(err, "listing open PRs")
	}

	// spawn activities to list modified fles in each open PR in parallel
	futureByPullNum := map[github.PullRequest]workflow.Future{}
	for _, pullRequest := range listOpenPRsResp.PullRequests {
		future := workflow.ExecuteActivity(ctx, p.RebaseActivites.GithubListModifiedFiles, activities.ListModifiedFilesRequest{
			Repo:        repo,
			PullRequest: pullRequest,
		})
		futureByPullNum[pullRequest] = future
	}

	// resolve the futures and rebase PR if needed
	rebaseFutures := []workflow.Future{}
	for pullRequest, future := range futureByPullNum {
		scope.Counter("open_prs").Inc(1)

		// list modified files should not fail due to ratelimit issues since our worker is ratelimited to only support API calls within our GH API budget per hour
		var result activities.ListModifiedFilesResponse
		listFilesErr := future.Get(ctx, &result)
		if listFilesErr != nil {
			logger.Error(ctx, "error listing modified files in PR", key.ErrKey, listFilesErr, key.PullNumKey, pullRequest.Number)
			continue
		}

		shouldRebase, err := shouldRebasePullRequest(root, result.FilePaths)

		// unlikey for error since we validate the WhenModified config at startup
		if err != nil {
			scope.Counter("filepath_match_err").Inc(1)
			logger.Error(ctx, "error matching filepaths in PR", key.ErrKey, err, key.PullNumKey, pullRequest.Number)
			continue
		}

		if !shouldRebase {
			continue
		}

		// spawn activity to rebase this PR and continue
		rebaseFutures = append(rebaseFutures, workflow.ExecuteActivity(ctx, p.RebaseActivites.SetPRRevision, activities.SetPRRevisionRequest{
			Repository:  repo,
			PullRequest: pullRequest,
		}))
	}

	// wait for rebase futures to resolve
	for _, future := range rebaseFutures {
		var resp activities.SetPRRevisionResponse
		err := future.Get(ctx, &resp)
		if err != nil {
			return errors.Wrap(err, "error making api call to set pr revision")
		}
	}

	return nil
}

func shouldRebasePullRequest(root terraform.Root, modifiedFiles []string) (bool, error) {

	// look at the filpaths for the root
	trackedFilesRelToRepoRoot := root.GetTrackedFilesRelativeToRepo()
	pm, err := fileutils.NewPatternMatcher(trackedFilesRelToRepoRoot)
	if err != nil {
		return false, errors.Wrap(err, "building file pattern matcher using when modified config")
	}

	for _, file := range modifiedFiles {
		match, err := pm.Matches(file)
		if err != nil {
			return false, errors.Wrap(err, "matching file path")
		}

		if !match {
			continue
		}

		return true, nil
	}

	return false, nil
}
