package prrevision

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/lyft/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

type diffInput struct {
	prs      []github.PullRequest
	rootMod  map[int]bool
	listFail map[int]bool
	setFail  map[int]bool
}

type diffOutcome struct {
	queues      map[int]string
	revisionSet map[int]bool
	failed      bool
}

func randomDiffInput(r *rand.Rand) diffInput {
	n := r.Intn(26)
	in := diffInput{rootMod: map[int]bool{}, listFail: map[int]bool{}, setFail: map[int]bool{}}
	for i := 1; i <= n; i++ {
		pr := github.PullRequest{Number: i, UpdatedAt: daysAgo(r.Intn(40)), IsAutomatedPR: r.Intn(4) == 0}
		in.prs = append(in.prs, pr)
		in.rootMod[i] = r.Intn(5) < 2
		in.listFail[i] = r.Intn(12) == 0
		in.setFail[i] = r.Intn(30) == 0
	}
	// newest-updated first, as GitHub returns them
	for i := 0; i < len(in.prs); i++ {
		for j := i + 1; j < len(in.prs); j++ {
			if in.prs[j].UpdatedAt.After(in.prs[i].UpdatedAt) {
				in.prs[i], in.prs[j] = in.prs[j], in.prs[i]
			}
		}
	}
	return in
}

func runDiff(t *testing.T, in diffInput, legacy bool) diffOutcome {
	t.Helper()
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()
	ga := &testGithubActivities{}
	ra := &testRevisionSetterActivities{}
	env.RegisterActivity(ga)
	env.RegisterActivity(ra)
	if legacy {
		env.OnGetVersion(concurrentPRRevisionChangeID, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	}

	var mu sync.Mutex
	out := diffOutcome{queues: map[int]string{}, revisionSet: map[int]bool{}}

	env.OnActivity(ga.GithubListPRs, mock.Anything, mock.Anything).Return(activities.ListPRsResponse{PullRequests: in.prs}, nil)
	env.OnActivity(ga.GithubListModifiedFiles, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.ListModifiedFilesRequest) (activities.ListModifiedFilesResponse, error) {
			mu.Lock()
			out.queues[request.PullRequest.Number] = activity.GetInfo(ctx).TaskQueue
			mu.Unlock()
			if in.listFail[request.PullRequest.Number] {
				return activities.ListModifiedFilesResponse{}, temporal.NewNonRetryableApplicationError("list failed", "ListFailed", nil)
			}
			if in.rootMod[request.PullRequest.Number] {
				return activities.ListModifiedFilesResponse{FilePaths: rootModifiedFiles}, nil
			}
			return activities.ListModifiedFilesResponse{FilePaths: rootNotModifiedFiles}, nil
		})
	env.OnActivity(ra.SetPRRevision, mock.Anything, mock.Anything).Return(
		func(ctx context.Context, request activities.SetPRRevisionRequest) error {
			mu.Lock()
			out.revisionSet[request.PullRequest.Number] = true
			mu.Unlock()
			if in.setFail[request.PullRequest.Number] {
				return temporal.NewNonRetryableApplicationError("set failed", "SetFailed", nil)
			}
			return nil
		})

	env.ExecuteWorkflow(func(ctx workflow.Context, r Request) error {
		return testSetMiminumValidRevisionForRootWorkflow(ctx, r, testCutOffDays)
	}, queueRoutingRequest())
	assert.True(t, env.IsWorkflowCompleted())
	failed := env.GetWorkflowError() != nil

	// snapshot under the lock, activities of a misbehaving workflow can still be running
	mu.Lock()
	defer mu.Unlock()
	snapshot := diffOutcome{queues: map[int]string{}, revisionSet: map[int]bool{}, failed: failed}
	for k, v := range out.queues {
		snapshot.queues[k] = v
	}
	for k, v := range out.revisionSet {
		snapshot.revisionSet[k] = v
	}
	return snapshot
}

// stickyModel is the original routing: the first stale or automated PR switches every later PR to the slow queue.
func stickyModel(prs []github.PullRequest) map[int]string {
	m, slow := map[int]string{}, false
	for _, pr := range prs {
		if pr.IsAutomatedPR || pr.UpdatedAt.Before(daysAgo(testCutOffDays)) {
			slow = true
		}
		m[pr.Number] = TaskQueue
		if slow {
			m[pr.Number] = SlowTaskQueue
		}
	}
	return m
}

func idealModel(prs []github.PullRequest) map[int]string {
	m := map[int]string{}
	for _, pr := range prs {
		m[pr.Number] = TaskQueue
		if pr.IsAutomatedPR || pr.UpdatedAt.Before(daysAgo(testCutOffDays)) {
			m[pr.Number] = SlowTaskQueue
		}
	}
	return m
}

// Runs the original and the new behavior over randomized inputs. Only routing may differ: the PRs that are
// checked and whether the workflow fails must be identical, and so must the PRs that get a revision set. The one
// exception is a failed revision, where the original returns at the first failure and abandons the rest.
func TestQueueRouting_OriginalAndNewAgreeOnOutcomes(t *testing.T) {
	r := rand.New(rand.NewSource(20261106)) //nolint:gosec // fixed seed so the random inputs are reproducible
	routingChanged := 0
	for i := 0; i < 200; i++ {
		in := randomDiffInput(r)
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			legacy := runDiff(t, in, true)
			current := runDiff(t, in, false)

			// every PR is checked exactly once in both versions
			assert.Len(t, legacy.queues, len(in.prs))
			assert.Len(t, current.queues, len(in.prs))

			// a PR gets a revision when it modifies the root, or when listing its files failed
			attempted := map[int]bool{}
			expectFailure := false
			for _, pr := range in.prs {
				if in.rootMod[pr.Number] || in.listFail[pr.Number] {
					attempted[pr.Number] = true
					if in.setFail[pr.Number] {
						expectFailure = true
					}
				}
			}
			assert.Equal(t, expectFailure, legacy.failed)
			assert.Equal(t, expectFailure, current.failed)

			// the new behavior waits for every revision to be attempted
			assert.Equal(t, attempted, current.revisionSet)
			if expectFailure {
				// the original returns at the first failed revision and abandons the ones still running,
				// so it can only have set a subset
				for n := range legacy.revisionSet {
					assert.True(t, current.revisionSet[n], "PR %d revised by the original but not the new behavior", n)
				}
			} else {
				assert.Equal(t, legacy.revisionSet, current.revisionSet)
			}
			// routing: original is the sticky model, new is each PR on its own merits
			assert.Equal(t, stickyModel(in.prs), legacy.queues)
			assert.Equal(t, idealModel(in.prs), current.queues)
		})
		if len(in.prs) > 0 && fmt.Sprint(stickyModel(in.prs)) != fmt.Sprint(idealModel(in.prs)) {
			routingChanged++
		}
	}
	// make sure the random inputs actually exercise the difference
	assert.Greater(t, routingChanged, 20, "random inputs rarely hit the routing difference")
}

// Many PRs in one workflow: nothing is dropped, every revision is set, no deadlock.
func TestQueueRouting_LargeNumberOfPRs(t *testing.T) {
	r := rand.New(rand.NewSource(7)) //nolint:gosec // fixed seed so the random inputs are reproducible
	in := diffInput{rootMod: map[int]bool{}, listFail: map[int]bool{}, setFail: map[int]bool{}}
	for i := 1; i <= 600; i++ {
		in.prs = append(in.prs, github.PullRequest{Number: i, UpdatedAt: daysAgo(i / 30), IsAutomatedPR: r.Intn(10) == 0})
		in.rootMod[i] = i%3 == 0
	}
	for _, legacy := range []bool{true, false} {
		out := runDiff(t, in, legacy)
		assert.False(t, out.failed)
		assert.Len(t, out.queues, 600)
		assert.Len(t, out.revisionSet, 200)
	}
}
