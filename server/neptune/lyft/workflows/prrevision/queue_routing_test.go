package prrevision

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/runatlantis/atlantis/server/neptune/lyft/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const testCutOffDays = 14

var (
	rootModifiedFiles    = []string{"test/dir2/main.tf"}
	rootNotModifiedFiles = []string{"other/dir/main.tf"}
)

func queueRoutingRequest() Request {
	return Request{
		Repo: github.Repo{Owner: "owner", Name: "test"},
		Root: terraform.Root{
			Path:         "test/dir2",
			TrackedFiles: raw.DefaultAutoPlanWhenModified,
		},
	}
}

type queueRoutingResult struct {
	// task queue each PR's GithubListModifiedFiles ran on, by PR number
	queues map[int]string
	// PRs SetPRRevision was called for
	revisionSet map[int]bool
}

// runQueueRouting runs the workflow with the given open PRs (in the order GitHub would return them),
// where every PR modifies the root, and reports where each activity ran.
func runQueueRouting(t *testing.T, prs []github.PullRequest, setup func(env *testsuite.TestWorkflowEnvironment)) queueRoutingResult {
	t.Helper()

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ra)

	if setup != nil {
		setup(env)
	}

	var mu sync.Mutex
	result := queueRoutingResult{queues: map[int]string{}, revisionSet: map[int]bool{}}

	env.OnActivity(ga.GithubListPRs, mock.Anything, mock.Anything).Return(activities.ListPRsResponse{
		PullRequests: prs,
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
			mu.Lock()
			defer mu.Unlock()
			result.queues[request.PullRequest.Number] = activity.GetInfo(ctx).TaskQueue
			return activities.ListModifiedFilesResponse{FilePaths: rootModifiedFiles}, nil
		})

	env.OnActivity(ra.SetPRRevision, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.SetPRRevisionRequest) error {
			mu.Lock()
			defer mu.Unlock()
			result.revisionSet[request.PullRequest.Number] = true
			return nil
		})

	env.ExecuteWorkflow(func(ctx workflow.Context, r Request) error {
		return testSetMiminumValidRevisionForRootWorkflow(ctx, r, testCutOffDays)
	}, queueRoutingRequest())

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
	return result
}

func daysAgo(days int) time.Time {
	return time.Now().UTC().AddDate(0, 0, -days)
}

// Each PR must be routed on its own merits. PRs are listed newest-updated first, and the first
// automated or stale PR must not drag the PRs listed after it onto the slow queue.
func TestQueueRouting_EachPRRoutedIndependently(t *testing.T) {
	cases := []struct {
		description string
		prs         []github.PullRequest
		expected    map[int]string
	}{
		{
			description: "automated PR first, recent PRs after it stay on the default queue",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(1), IsAutomatedPR: true},
				{Number: 2, UpdatedAt: daysAgo(2)},
				{Number: 3, UpdatedAt: daysAgo(3)},
			},
			expected: map[int]string{1: SlowTaskQueue, 2: TaskQueue, 3: TaskQueue},
		},
		{
			description: "automated PR in the middle",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(1)},
				{Number: 2, UpdatedAt: daysAgo(2), IsAutomatedPR: true},
				{Number: 3, UpdatedAt: daysAgo(3)},
			},
			expected: map[int]string{1: TaskQueue, 2: SlowTaskQueue, 3: TaskQueue},
		},
		{
			description: "stale PR before a recent PR (input not sorted)",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(30)},
				{Number: 2, UpdatedAt: daysAgo(2)},
			},
			expected: map[int]string{1: SlowTaskQueue, 2: TaskQueue},
		},
		{
			description: "recent PRs followed by stale PRs",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(1)},
				{Number: 2, UpdatedAt: daysAgo(20)},
				{Number: 3, UpdatedAt: daysAgo(40)},
			},
			expected: map[int]string{1: TaskQueue, 2: SlowTaskQueue, 3: SlowTaskQueue},
		},
		{
			description: "all recent",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(1)},
				{Number: 2, UpdatedAt: daysAgo(2)},
			},
			expected: map[int]string{1: TaskQueue, 2: TaskQueue},
		},
		{
			description: "all automated",
			prs: []github.PullRequest{
				{Number: 1, UpdatedAt: daysAgo(1), IsAutomatedPR: true},
				{Number: 2, UpdatedAt: daysAgo(2), IsAutomatedPR: true},
			},
			expected: map[int]string{1: SlowTaskQueue, 2: SlowTaskQueue},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			result := runQueueRouting(t, c.prs, nil)
			assert.Equal(t, c.expected, result.queues)
			// every PR modifies the root, so every PR must still get its minimum revision set
			for _, pr := range c.prs {
				assert.True(t, result.revisionSet[pr.Number], "revision not set for PR %d", pr.Number)
			}
		})
	}
}

// A PR stuck on the slow queue must not hold up setting the revision for a recent PR listed after it.
// The slow PR's listing only finishes once the recent PR's revision has been set, so a workflow that
// handles PRs strictly in list order cannot make progress and the wait below times out.
func TestQueueRouting_SlowPRDoesNotBlockRecentPRRevision(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ra)

	slowPR := github.PullRequest{Number: 1, UpdatedAt: daysAgo(1), IsAutomatedPR: true}
	recentPR := github.PullRequest{Number: 2, UpdatedAt: daysAgo(2)}

	recentRevisionSet := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	timedOut := false

	env.OnActivity(ga.GithubListPRs, mock.Anything, mock.Anything).Return(activities.ListPRsResponse{
		PullRequests: []github.PullRequest{slowPR, recentPR},
	}, nil)

	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
			if request.PullRequest.Number != slowPR.Number {
				return activities.ListModifiedFilesResponse{FilePaths: rootModifiedFiles}, nil
			}
			select {
			case <-recentRevisionSet:
			case <-time.After(3 * time.Second):
				mu.Lock()
				timedOut = true
				mu.Unlock()
			}
			return activities.ListModifiedFilesResponse{FilePaths: rootNotModifiedFiles}, nil
		})

	env.OnActivity(ra.SetPRRevision, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.SetPRRevisionRequest) error {
			if request.PullRequest.Number == recentPR.Number {
				once.Do(func() { close(recentRevisionSet) })
			}
			return nil
		})

	env.ExecuteWorkflow(func(ctx workflow.Context, r Request) error {
		return testSetMiminumValidRevisionForRootWorkflow(ctx, r, testCutOffDays)
	}, queueRoutingRequest())

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	mu.Lock()
	defer mu.Unlock()
	assert.False(t, timedOut, "recent PR's revision was only set after the slow PR finished listing its files")
}

// A failure setting one PR's revision must still fail the workflow.
func TestQueueRouting_SetPRRevisionErrorFailsWorkflow(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ra)

	prs := []github.PullRequest{
		{Number: 1, UpdatedAt: daysAgo(1)},
		{Number: 2, UpdatedAt: daysAgo(2)},
	}

	env.OnActivity(ga.GithubListPRs, mock.Anything, mock.Anything).Return(activities.ListPRsResponse{PullRequests: prs}, nil)
	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, mock.Anything).Return(
		activities.ListModifiedFilesResponse{FilePaths: rootModifiedFiles}, nil)
	env.OnActivity(ra.SetPRRevision, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.SetPRRevisionRequest) error {
			if request.PullRequest.Number == 2 {
				return assert.AnError
			}
			return nil
		})

	env.ExecuteWorkflow(func(ctx workflow.Context, r Request) error {
		return testSetMiminumValidRevisionForRootWorkflow(ctx, r, testCutOffDays)
	}, queueRoutingRequest())

	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

// Workflows that started before the fix replay with the original behavior. This pins the legacy path
// so the version gate keeps replaying old histories the way they originally ran, including the
// original routing, which is why this asserts the old (buggy) queue assignment.
func TestQueueRouting_LegacyVersionKeepsOriginalBehavior(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 1, UpdatedAt: daysAgo(1), IsAutomatedPR: true},
		{Number: 2, UpdatedAt: daysAgo(2)},
	}

	result := runQueueRouting(t, prs, func(env *testsuite.TestWorkflowEnvironment) {
		env.OnGetVersion(concurrentPRRevisionChangeID, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	})

	assert.Equal(t, map[int]string{1: SlowTaskQueue, 2: SlowTaskQueue}, result.queues)
	assert.True(t, result.revisionSet[1])
	assert.True(t, result.revisionSet[2])
}
