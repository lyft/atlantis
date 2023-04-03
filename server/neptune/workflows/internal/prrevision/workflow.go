package prrevision

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
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/prrevision/version"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// PRs updated before n days will be processed in the LowThroughPutTaskQueue
	ThroughputCutOffDays = 30

	// preserving the original tq name to account for existing workflow executions
	TaskQueue     = "pr_revision"
	SlowTaskQueue = "pr_revision_slow"

	RetryCount          = 3
	StartToCloseTimeout = 30 * time.Second
)

type Request struct {
	Repo     github.Repo
	Root     terraform.Root
	Revision string
}

type setterActivities interface {
	SetPRRevision(ctx context.Context, request activities.SetPRRevisionRequest) error
}

type githubActivities interface {
	GithubListPRs(ctx context.Context, request activities.ListPRsRequest) (activities.ListPRsResponse, error)
	GithubListModifiedFiles(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error)
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
		GithubActivities:         ga,
		RevisionSetterActivities: ra,
		Scope:                    metrics.NewScope(ctx),
	}

	return runner.Run(ctx, request)
}

type Runner struct {
	GithubActivities         githubActivities
	RevisionSetterActivities setterActivities
	Scope                    metrics.Scope
}

func (r *Runner) Run(ctx workflow.Context, request Request) error {
	prs, err := r.listOpenPRs(ctx, request.Repo)
	if err != nil {
		return err
	}

	version := workflow.GetVersion(ctx, version.MultiQueue, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return r.setRevisionV1(ctx, request, prs)
	}

	oldPRs, newPRs := r.partitionPRsByDateModified(workflow.Now(ctx), prs)

	// handle new PRs in the high throughput task queue
	r.Scope.SubScope("open_prs").Counter("new").Inc(int64(len(prs)))
	if err := r.setRevision(ctx, TaskQueue, request, newPRs); err != nil {
		return errors.Wrap(err, "setting minimum revision for new prs modifiying root")
	}

	// handle old PRs in the low throughput task queue
	r.Scope.SubScope("open_prs").Counter("old").Inc(int64(len(prs)))
	if err := r.setRevision(ctx, SlowTaskQueue, request, oldPRs); err != nil {
		return errors.Wrap(err, "setting minimum revision for old prs modifiying root")
	}

	return nil
}

func (r *Runner) listOpenPRs(ctx workflow.Context, repo github.Repo) ([]github.PullRequest, error) {
	var resp activities.ListPRsResponse
	err := workflow.ExecuteActivity(ctx, r.GithubActivities.GithubListPRs, activities.ListPRsRequest{
		Repo:  repo,
		State: github.OpenPullRequest,
	}).Get(ctx, &resp)
	if err != nil {
		return []github.PullRequest{}, errors.Wrap(err, "listing open PRs")
	}

	return resp.PullRequests, nil
}

func (r *Runner) setRevisionV1(ctx workflow.Context, request Request, prs []github.PullRequest) error {
	r.Scope.Counter("open_prs").Inc(int64(len(prs)))

	// schedule all PRs in the High Throughput TQ
	if err := r.setRevision(ctx, TaskQueue, request, prs); err != nil {
		return errors.Wrap(err, "setting minimum revision for pr modifiying root")
	}
	return nil
}

func (r *Runner) setRevision(ctx workflow.Context, taskQueue string, req Request, prs []github.PullRequest) error {
	// spawn activities to list modified files in each open PR async
	futuresByPullNum := map[github.PullRequest]workflow.Future{}
	for _, pr := range prs {
		futuresByPullNum[pr] = r.listModifiedFilesFuture(ctx, taskQueue, activities.ListModifiedFilesRequest{
			Repo:        req.Repo,
			PullRequest: pr,
		})
	}

	// resolve the listModifiedFiles fututes and spawn activities to set minimum revision for PR if needed
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

func (r *Runner) listModifiedFilesFuture(ctx workflow.Context, taskQueue string, req activities.ListModifiedFilesRequest) workflow.Future {
	opts := workflow.GetActivityOptions(ctx)
	opts.TaskQueue = taskQueue
	ctx = workflow.WithActivityOptions(ctx, opts)

	return workflow.ExecuteActivity(ctx, r.GithubActivities.GithubListModifiedFiles, req)
}

func (r *Runner) setRevisionForPR(ctx workflow.Context, req Request, pull github.PullRequest, future workflow.Future) workflow.Future {
	// let's be preventive and set minimum revision for this PR if this listModifiedFiles fails after 3 attempts
	var result activities.ListModifiedFilesResponse
	if err := future.Get(ctx, &result); err != nil {
		logger.Error(ctx, "error listing modified files in PR", key.ErrKey, err, key.PullNumKey, pull.Number)
		return r.setMinRevision(ctx, req, pull)
	}

	// should not fail unless the TrackedFiles config is invalid which is validated on startup
	// let's be preventive and set minimum revision for this PR if file path match errors out
	rootModified, err := isRootModified(req.Root, result.FilePaths)
	if err != nil {
		logger.Error(ctx, "error matching file paths in PR", key.ErrKey, err, key.PullNumKey, pull.Number)
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

func (r *Runner) partitionPRsByDateModified(now time.Time, prs []github.PullRequest) ([]github.PullRequest, []github.PullRequest) {
	nDaysAgo := now.AddDate(0, 0, -ThroughputCutOffDays)
	oldPRs, newPRs := []github.PullRequest{}, []github.PullRequest{}
	for _, pr := range prs {
		if pr.UpdatedAt.Before(nDaysAgo) {
			oldPRs = append(oldPRs, pr)
		} else {
			newPRs = append(newPRs, pr)
		}
	}

	return oldPRs, newPRs
}
