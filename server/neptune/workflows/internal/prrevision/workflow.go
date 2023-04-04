package prrevision

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/pkg/errors"
	key "github.com/runatlantis/atlantis/server/neptune/context"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/metrics"
	githubInternal "github.com/runatlantis/atlantis/server/neptune/workflows/internal/prrevision/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/prrevision/version"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// preserving the original tq name to account for existing workflow executions
	TaskQueue     = "pr_revision"
	SlowTaskQueue = "pr_revision_slow"

	RetryCount          = 3
	StartToCloseTimeout = 30 * time.Second
)

type Request struct {
	Repo                     github.Repo
	Root                     terraform.Root
	Revision                 string
	SlowProcessingCutOffDays int
}

type setterActivities interface {
	SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) error
}

type activityExecutor interface {
	GithubListModifiedFiles(ctx workflow.Context, taskqueue string, request activities.ListModifiedFilesRequest) workflow.Future
	GithubListPRs(ctx workflow.Context, request activities.ListPRsRequest) workflow.Future
}

func Workflow(ctx workflow.Context, request Request) error {
	// GH API calls should not hit ratelimit issues since we cap the TaskQueueActivitiesPerSecond for the min revison setter TQ such that it's within our GH API budget
	// Configuring both GH API calls and PRSetRevision calls to 3 retries before failing
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: StartToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: RetryCount,
		},
	})

	var ga *activities.Github
	var ra *activities.RevsionSetter
	runner := &Runner{
		GithubActivityExecutor: &githubInternal.ActivityExecutor{
			GithubActivities: ga,
		},
		RevisionSetterActivities: ra,
		Scope:                    metrics.NewScope(ctx),
		SlowProcessingCutOffDays: request.SlowProcessingCutOffDays,
	}

	return runner.Run(ctx, request)
}

type Runner struct {
	GithubActivityExecutor   activityExecutor
	RevisionSetterActivities setterActivities
	Scope                    metrics.Scope
	SlowProcessingCutOffDays int
}

func (r *Runner) Run(ctx workflow.Context, request Request) error {
	// sorted in descending order by date modified
	prs, err := r.listOpenPRs(ctx, request.Repo)
	if err != nil {
		return err
	}

	r.Scope.Counter("open_prs").Inc(int64(len(prs)))
	if err := r.setRevision(ctx, request, prs); err != nil {
		return errors.Wrap(err, "setting minimum revision for pr modifiying root")
	}

	return nil
}

func (r *Runner) listOpenPRs(ctx workflow.Context, repo github.Repo) ([]github.PullRequest, error) {
	var resp activities.ListPRsResponse
	err := r.GithubActivityExecutor.GithubListPRs(ctx, activities.ListPRsRequest{
		Repo:      repo,
		State:     github.OpenPullRequest,
		SortKey:   github.Updated,
		Direction: github.Descending,
	}).Get(ctx, &resp)
	if err != nil {
		return []github.PullRequest{}, errors.Wrap(err, "listing open PRs")
	}

	return resp.PullRequests, nil
}

func (r *Runner) setRevision(ctx workflow.Context, req Request, prs []github.PullRequest) error {
	futuresByPullNum := r.listModifiedFilesAsync(ctx, req, prs)

	// sorted by date modified ensures we resolve the futures for activities in the default task queue before resolving activities in the slow task queue
	setRevisionFutures := []workflow.Future{}
	for _, pr := range prs {
		if future := r.setRevisionForPR(ctx, req, pr, futuresByPullNum[pr]); future != nil {
			setRevisionFutures = append(setRevisionFutures, future)
		}
	}

	// wait to resolve futures for setting minimum revision
	for _, future := range setRevisionFutures {
		if err := future.Get(ctx, nil); err != nil {
			return errors.Wrap(err, "error setting pr revision")
		}
	}

	return nil
}

func (r *Runner) listModifiedFilesAsync(ctx workflow.Context, req Request, prs []github.PullRequest) map[github.PullRequest]workflow.Future {
	now := workflow.Now(ctx)
	taskQueue := TaskQueue
	futuresByPullNum := map[github.PullRequest]workflow.Future{}

	oldPrsCount := 0
	for _, pr := range prs {
		// if new version of workflow and pr is not updated within ThroughputCutOffDays, change the tq
		version := workflow.GetVersion(ctx, version.MultiQueue, workflow.DefaultVersion, 1)
		if version != workflow.DefaultVersion && !r.isPrUpdatedWithinDays(now, pr, r.SlowProcessingCutOffDays) {
			taskQueue = SlowTaskQueue
			oldPrsCount++
		}

		futuresByPullNum[pr] = r.GithubActivityExecutor.GithubListModifiedFiles(ctx, taskQueue, activities.ListModifiedFilesRequest{
			Repo:        req.Repo,
			PullRequest: pr,
		})
	}

	newPrsCount := len(prs) - oldPrsCount
	r.Scope.SubScope("open_prs").Counter(fmt.Sprintf("less_than_%d_days", r.SlowProcessingCutOffDays)).Inc(int64(newPrsCount))
	return futuresByPullNum
}

func (r *Runner) isPrUpdatedWithinDays(now time.Time, pr github.PullRequest, days int) bool {
	nDaysAgo := now.AddDate(0, 0, -days)
	return pr.UpdatedAt.After(nDaysAgo)
}

func (r *Runner) setRevisionForPR(ctx workflow.Context, req Request, pull github.PullRequest, future workflow.Future) workflow.Future {
	// let's be preventive and set minimum revision for this PR if this listModifiedFiles fails after 3 attempts
	var result activities.ListModifiedFilesResponse
	if err := future.Get(ctx, &result); err != nil {
		workflow.GetLogger(ctx).Error("error listing modified files in PR", key.ErrKey, err, key.PullNumKey, pull.Number)
		return r.setMinRevision(ctx, req, pull)
	}

	// should not fail unless the TrackedFiles config is invalid which is validated on startup
	// let's be preventive and set minimum revision for this PR if file path match errors out
	rootModified, err := isRootModified(req.Root, result.FilePaths)
	if err != nil {
		workflow.GetLogger(ctx).Error("error matching file paths in PR", key.ErrKey, err, key.PullNumKey, pull.Number)
		return r.setMinRevision(ctx, req, pull)
	}

	if rootModified {
		return r.setMinRevision(ctx, req, pull)
	}

	return nil
}

func (r *Runner) setMinRevision(ctx workflow.Context, req Request, pull github.PullRequest) workflow.Future {
	return workflow.ExecuteActivity(ctx, r.RevisionSetterActivities.SetPRRevision, activities.SetPRRevisionRequest{
		Repository:  req.Repo,
		PullRequest: pull,
		Revision:    req.Revision,
	})
}

func isRootModified(root terraform.Root, modifiedFiles []string) (bool, error) {
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
