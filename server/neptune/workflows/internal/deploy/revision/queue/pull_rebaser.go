package queue

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/config/logger"
	"go.temporal.io/sdk/workflow"
)

// GH API Ratelimit resets every hour, 2 hours should be enough for the GH API call failures to autoresolve
const RebaseGithubScheduleToCloseTimeout = 2 * time.Hour

type githubRebaseActivities interface {
	GithubListOpenPRs(ctx context.Context, request activities.ListOpenPRsRequest) (activities.ListOpenPRsResponse, error)
	GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error)
}

type buildNotifyActivities interface {
	BuildNotifyRebasePR(ctx context.Context, request activities.BuildNotifyRebasePRRequest) (activities.BuildNotifyRebasePRResponse, error)
}

type PullRebaser struct {
	GithubActivities      githubRebaseActivities
	BuildNotifyActivities buildNotifyActivities
}

func (p *PullRebaser) RebaseOpenPRsForRoot(ctx workflow.Context, repo github.Repo, root terraform.Root) error {
	originalOpts := workflow.GetActivityOptions(ctx)

	// Configure github specific ScheduleToClose timeout for GH API call failures to autoresolve
	// TODO: Add alarms on GithubListOpenPRs and GithubListModifiedFiles API call failures
	ghOpts := originalOpts
	ghOpts.ScheduleToCloseTimeout = RebaseGithubScheduleToCloseTimeout
	ctx = workflow.WithActivityOptions(ctx, ghOpts)

	// list open PRs
	var listOpenPRsResp activities.ListOpenPRsResponse
	err := workflow.ExecuteActivity(ctx, p.GithubActivities.GithubListOpenPRs, activities.ListOpenPRsRequest{
		Repo: repo,
	}).Get(ctx, &listOpenPRsResp)
	if err != nil {
		return errors.Wrap(err, "listing open PRs")
	}

	// spawn activities to list modified fles in each open PR in parallel
	futureByPullNum := map[github.PullRequest]workflow.Future{}
	for _, pullRequest := range listOpenPRsResp.PullRequests {
		future := workflow.ExecuteActivity(ctx, p.GithubActivities.GithubListModifiedFiles, activities.ListModifiedFilesRequest{
			Repo:        repo,
			PullRequest: pullRequest,
		})
		futureByPullNum[pullRequest] = future
	}

	// resolve the futures and check if an open PR needs to be rebased
	prsToRebase := []github.PullRequest{}
	for pullRequest, future := range futureByPullNum {
		var result activities.ListModifiedFilesResponse

		// list modified files should not fail unless we hit the ratelimit which should autoresolve once our ratelimit revives in an hour max
		// If it errors out due to any other reason, let's be preventive and rebase this PR as well
		listFilesErr := future.Get(ctx, &result)
		if listFilesErr != nil {
			logger.Error(ctx, "error listing modified files in PR", key.ErrKey, listFilesErr, "pull_num", pullRequest.Number)
			prsToRebase = append(prsToRebase, pullRequest)
			continue
		}

		shouldRebase, err := shouldRebasePullRequest(root, result.FilePaths)

		// unlikey for error since we validate the WhenModified config at startup
		// if it errors out, let's be preventive and rebase this PR as well
		if err != nil {
			logger.Error(ctx, "error matching filepaths in PR", key.ErrKey, err, "pull_num", pullRequest.Number)
			prsToRebase = append(prsToRebase, pullRequest)
			continue
		}

		if shouldRebase {
			prsToRebase = append(prsToRebase, pullRequest)
		}
	}

	// skip buildnotify call if no prs to rebase
	if len(prsToRebase) == 0 {
		return nil
	}

	// reset workflow options
	ctx = workflow.WithActivityOptions(ctx, originalOpts)
	futureByPullNum = map[github.PullRequest]workflow.Future{}
	for _, pr := range prsToRebase {
		futureByPullNum[pr] = workflow.ExecuteActivity(ctx, p.BuildNotifyActivities.BuildNotifyRebasePR, activities.BuildNotifyRebasePRRequest{
			Repository:  repo,
			PullRequest: pr,
		})
	}

	// TODO: Add alarms on BuildNotifyRebasePR activity failures
	for pr, future := range futureByPullNum {
		var resp activities.BuildNotifyRebasePRResponse
		err := future.Get(ctx, &resp)
		if err != nil {
			logger.Error(ctx, "error making API call to buildnotify", key.ErrKey, err, "pull_num", pr.Number)
		}
	}

	return nil
}

func shouldRebasePullRequest(root terraform.Root, modifiedFiles []string) (bool, error) {
	var whenModifiedRelToRepoRoot []string
	for _, wm := range root.WhenModified {
		wm = strings.TrimSpace(wm)
		// An exclusion uses a '!' at the beginning. If it's there, we need
		// to remove it, then add in the project path, then add it back.
		exclusion := false
		if wm != "" && wm[0] == '!' {
			wm = wm[1:]
			exclusion = true
		}

		// Prepend project dir to when modified patterns because the patterns
		// are relative to the project dirs but our list of modified files is
		// relative to the repo root.
		wmRelPath := filepath.Join(root.Path, wm)
		if exclusion {
			wmRelPath = "!" + wmRelPath
		}
		whenModifiedRelToRepoRoot = append(whenModifiedRelToRepoRoot, wmRelPath)
	}

	// look at the filpaths for the root
	pm, err := fileutils.NewPatternMatcher(whenModifiedRelToRepoRoot)
	if err != nil {
		return false, errors.Wrap(err, "building file pattern matcher using when modified config")
	}

	for _, file := range modifiedFiles {
		match, err := pm.Matches(file)
		if err != nil || !match {
			continue
		}

		// only 1 match needed
		return true, nil
	}

	return false, nil
}
