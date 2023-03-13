package revisionsetter

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
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	GithubRetryCount    = 3
	StartToCloseTimeout = 10 * time.Second
)

type Request struct {
	Repo github.Repo
	Root terraform.Root
}

type revisionSetterActivities interface {
	SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) (activities.SetPRRevisionResponse, error)
	GithubListOpenPRs(ctx context.Context, request activities.ListOpenPRsRequest) (activities.ListOpenPRsResponse, error)
	GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error)
}

func Workflow(ctx workflow.Context, request Request) error {
	// GH API calls should not hit ratelimit issues since we cap the TaskQueueActivitiesPerSecond for the rebase TQ
	// such that it's within our GH API budget
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: GithubRetryCount,
		},
	})

	var r revisionSetterActivities
	return SetMiminumValidRevisionForRoot(ctx, request, r)
}

func SetMiminumValidRevisionForRoot(ctx workflow.Context, request Request, r revisionSetterActivities) error {
	prs, err := listOpenPRs(ctx, request.Repo, r)
	if err != nil {
		return err
	}

	scope := metrics.NewScope(ctx)
	scope.Counter("open_prs").Inc(int64(len(prs)))

	setMinRevFutures, err := setMinRevisionForPrsModifiyingRoot(ctx, request, prs, r, scope)
	if err != nil {
		return errors.Wrap(err, "setting minimum revision for pr modifiying root")
	}

	// wait to resolve futures for setting minimum revision
	for _, future := range setMinRevFutures {
		var resp activities.SetPRRevisionResponse
		err := future.Get(ctx, &resp)
		if err != nil {
			return errors.Wrap(err, "error setting pr revision")
		}
	}

	return nil
}

func listOpenPRs(ctx workflow.Context, repo github.Repo, a revisionSetterActivities) ([]github.PullRequest, error) {
	var resp activities.ListOpenPRsResponse
	err := workflow.ExecuteActivity(ctx, a.GithubListOpenPRs, activities.ListOpenPRsRequest{
		Repo: repo,
	}).Get(ctx, &resp)
	if err != nil {
		return []github.PullRequest{}, errors.Wrap(err, "listing open PRs")
	}

	return resp.PullRequests, nil
}

func setMinRevisionForPrsModifiyingRoot(ctx workflow.Context, req Request, prs []github.PullRequest, a revisionSetterActivities, scope metrics.Scope) ([]workflow.Future, error) {
	// spawn activities to list modified files in each open PR async
	futuresByPullNum := map[github.PullRequest]workflow.Future{}
	for _, pr := range prs {
		futuresByPullNum[pr] = workflow.ExecuteActivity(ctx, a.GithubListModifiedFiles, activities.ListModifiedFilesRequest{
			Repo:        req.Repo,
			PullRequest: pr,
		})
	}

	// resolve the futures and set minimum revision for PR if needed
	futures := []workflow.Future{}
	for _, pr := range prs {
		future := futuresByPullNum[pr]

		// let's be preventive and rebase this PR if this call fails after 3 attempts
		var result activities.ListModifiedFilesResponse
		listFilesErr := future.Get(ctx, &result)
		if listFilesErr != nil {
			logger.Error(ctx, "error listing modified files in PR", key.ErrKey, listFilesErr, key.PullNumKey, pr.Number)
			futures = append(futures, workflow.ExecuteActivity(ctx, a.SetPRRevision, activities.SetPRRevisionRequest{
				Repository:  req.Repo,
				PullRequest: pr,
			}))
			continue
		}

		setMinRevision, err := shouldSetMinimumRevisionForPR(req.Root, result.FilePaths)
		if err != nil {
			return []workflow.Future{}, errors.Wrap(err, "error matching filepaths in PR")
		}

		if !setMinRevision {
			continue
		}

		// spawn activity to rebase this PR and continue
		futures = append(futures, workflow.ExecuteActivity(ctx, a.SetPRRevision, activities.SetPRRevisionRequest{
			Repository:  req.Repo,
			PullRequest: pr,
		}))
	}

	return futures, nil
}

func shouldSetMinimumRevisionForPR(root terraform.Root, modifiedFiles []string) (bool, error) {
	// look at the filepaths for the root
	trackedFilesRelToRepoRoot := root.GetTrackedFilesRelativeToRepo()
	pm, err := fileutils.NewPatternMatcher(trackedFilesRelToRepoRoot)
	if err != nil {
		return false, errors.Wrap(err, "building file pattern matcher using tracked files config")
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
