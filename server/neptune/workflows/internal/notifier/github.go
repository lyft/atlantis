package notifier

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github/markdown"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/terraform/state"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	KeyDelim       = "_"
	CompleteStatus = "completed"
)

type checksActivities interface {
	GithubUpdateCheckRun(ctx context.Context, request activities.UpdateCheckRunRequest) (activities.UpdateCheckRunResponse, error)
	GithubCreateCheckRun(ctx context.Context, request activities.CreateCheckRunRequest) (activities.CreateCheckRunResponse, error)
}

// GithubCheckRunCache manages the lifecycle of a given check run.  A check run is evicted
// from the cache when it is marked completed.  This allows consumers to not have to worry about
// passing check run ids around or determining which github api to call.
type GithubCheckRunCache struct {
	// state is mutable
	deploymentCheckRunCache map[string]int64
	activities              checksActivities
}

func NewGithubCheckRunCache(activities checksActivities) *GithubCheckRunCache {
	return &GithubCheckRunCache{
		deploymentCheckRunCache: map[string]int64{},
		activities:              activities,
	}
}

type GithubCheckRunRequest struct {
	Title   string
	Sha     string
	Repo    github.Repo
	State   github.CheckRunState
	Actions []github.CheckRunAction
	Summary string
}

func (c *GithubCheckRunCache) CreateOrUpdate(ctx workflow.Context, deploymentID string, request GithubCheckRunRequest) (int64, error) {
	key := deploymentID + KeyDelim + request.Title
	checkRunID, ok := c.deploymentCheckRunCache[key]

	// if we haven't created one, let's do so now
	if !ok {
		resp, err := c.load(ctx, deploymentID, request)
		if err != nil {
			return 0, err
		}
		c.deploymentCheckRunCache[key] = resp.ID
		c.deleteIfCompleted(resp.Status, key)

		return resp.ID, nil
	}

	// update existing checks
	resp, err := c.update(ctx, deploymentID, request, checkRunID)
	if err != nil {
		return 0, err
	}
	c.deleteIfCompleted(resp.Status, key)

	return checkRunID, nil
}

// if the check is complete, let's remove it from the map since we don't want to be updating
// complete checks going forward.
func (c *GithubCheckRunCache) deleteIfCompleted(status, key string) {
	if status == CompleteStatus {
		delete(c.deploymentCheckRunCache, key)
	}
}

func (c *GithubCheckRunCache) update(ctx workflow.Context, externalID string, request GithubCheckRunRequest, checkRunID int64) (activities.UpdateCheckRunResponse, error) {
	updateCheckRunRequest := activities.UpdateCheckRunRequest{
		Title:      request.Title,
		Repo:       request.Repo,
		State:      request.State,
		Actions:    request.Actions,
		Summary:    request.Summary,
		ID:         checkRunID,
		ExternalID: externalID,
	}

	var resp activities.UpdateCheckRunResponse
	err := workflow.ExecuteActivity(ctx, c.activities.GithubUpdateCheckRun, updateCheckRunRequest).Get(ctx, &resp)
	if err != nil {
		return resp, errors.Wrapf(err, "updating check run with id: %d", checkRunID)
	}
	return resp, nil
}

func (c *GithubCheckRunCache) load(ctx workflow.Context, externalID string, request GithubCheckRunRequest) (activities.CreateCheckRunResponse, error) {
	createCheckRunRequest := activities.CreateCheckRunRequest{
		Title:      request.Title,
		Sha:        request.Sha,
		Repo:       request.Repo,
		State:      request.State,
		Actions:    request.Actions,
		Summary:    request.Summary,
		ExternalID: externalID,
	}

	var resp activities.CreateCheckRunResponse
	err := workflow.ExecuteActivity(ctx, c.activities.GithubCreateCheckRun, createCheckRunRequest).Get(ctx, &resp)
	if err != nil {
		return resp, errors.Wrap(err, "creating check run")
	}
	workflow.GetLogger(ctx).Debug("created checkrun with id", "checkRunID", resp.ID)
	return resp, nil
}

// used so we can test dependencies in isolation
type checkRunClient interface {
	CreateOrUpdate(ctx workflow.Context, deploymentID string, request GithubCheckRunRequest) (int64, error)
}

type CheckRunNotifier struct {
	CheckRunSessionCache checkRunClient
	Mode                 terraform.WorkflowMode
}

type Info struct {
	ID       uuid.UUID
	Commit   github.Commit
	RootName string
	Repo     github.Repo
}

func (n *CheckRunNotifier) Notify(ctx workflow.Context, info Info, workflowState *state.Workflow) error {
	return errors.Wrap(n.updateCheckRun(ctx, workflowState, info), "updating check run")
}

func (n *CheckRunNotifier) updateCheckRun(ctx workflow.Context, workflowState *state.Workflow, info Info) error {
	summary := markdown.RenderWorkflowStateTmpl(workflowState)
	checkRunState := determineCheckRunState(workflowState)

	var title string
	if n.Mode == terraform.Deploy {
		title = BuildDeployCheckRunTitle(info.RootName)
	} else {
		title = BuildPRCheckRunTitle(info.RootName)
	}

	request := GithubCheckRunRequest{
		Title:   title,
		Sha:     info.Commit.Revision,
		State:   checkRunState,
		Repo:    info.Repo,
		Summary: summary,
	}

	if workflowState.Apply != nil {
		// add any actions pertaining to the apply job
		for _, a := range workflowState.Apply.GetActions().Actions {
			request.Actions = append(request.Actions, a.ToGithubCheckRunAction())
		}
	}

	// cap our retries for non-terminal states to allow for at least some progress
	if checkRunState != github.CheckRunFailure && checkRunState != github.CheckRunSuccess {
		ctx = workflow.WithRetryPolicy(ctx, temporal.RetryPolicy{
			MaximumAttempts: 3,
		})
	}

	_, err := n.CheckRunSessionCache.CreateOrUpdate(ctx, info.ID.String(), request)
	return err
}

func determineCheckRunState(workflowState *state.Workflow) github.CheckRunState {
	if waitingForActionOn(workflowState.Plan) || waitingForActionOn(workflowState.Apply) {
		return github.CheckRunActionRequired
	}

	if workflowState.Result.Status != state.CompleteWorkflowStatus {
		return github.CheckRunPending
	}

	if workflowState.Result.Reason == state.SuccessfulCompletionReason {
		return github.CheckRunSuccess
	}

	if workflowState.Result.Reason == state.SkippedCompletionReason {
		return github.CheckRunSkipped
	}

	timeouts := []state.WorkflowCompletionReason{
		state.TimeoutError,
		state.ActivityDurationTimeoutError,
		state.HeartbeatTimeoutError,
		state.SchedulingTimeoutError,
	}

	for _, t := range timeouts {
		if workflowState.Result.Reason == t {
			return github.CheckRunTimeout
		}
	}

	return github.CheckRunFailure
}

func waitingForActionOn(job *state.Job) bool {
	return job != nil && job.Status == state.WaitingJobStatus && len(job.OnWaitingActions.Actions) > 0
}

func BuildDeployCheckRunTitle(rootName string) string {
	return fmt.Sprintf("atlantis/deploy: %s", rootName)
}

func BuildPRCheckRunTitle(rootName string) string {
	return fmt.Sprintf("atlantis/pr: %s", rootName)
}
