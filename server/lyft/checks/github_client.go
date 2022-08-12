package checks

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v45/github"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

const (
	// Reference: https://github.com/github/docs/issues/3765
	maxChecksOutputLength = 65535
)

// github checks conclusion
type ChecksConclusion int

const (
	Neutral ChecksConclusion = iota
	TimedOut
	ActionRequired
	Cancelled
	Failure
	Success
)

func (e ChecksConclusion) String() string {
	switch e {
	case Neutral:
		return "neutral"
	case TimedOut:
		return "timed_out"
	case ActionRequired:
		return "action_required"
	case Cancelled:
		return "cancelled"
	case Failure:
		return "failure"
	case Success:
		return "success"
	}
	return ""
}

// github checks status
type CheckStatus int

const (
	Queued CheckStatus = iota
	InProgress
	Completed
)

func (e CheckStatus) String() string {
	switch e {
	case Queued:
		return "queued"
	case InProgress:
		return "in_progress"
	case Completed:
		return "completed"
	}
	return ""
}

// [WENGINES-4643] TODO: Remove this wrapper and add checks implementation to UpdateStatus() directly after github checks is stable
type ChecksClientWrapper struct {
	*vcs.GithubClient
	FeatureAllocator feature.Allocator
	Logger           logging.Logger
}

func (c *ChecksClientWrapper) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) (string, error) {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         request.Repo.FullName,
		PullCreationTime: request.PullCreationTime,
	})
	if err != nil {
		c.Logger.ErrorContext(ctx, fmt.Sprintf("unable to allocate for feature: %s", feature.GithubChecks), map[string]interface{}{
			"error": err.Error(),
		})
	}

	if !shouldAllocate {
		return c.GithubClient.UpdateStatus(ctx, request)
	}

	// Empty status ID means we create a new check run
	if request.StatusId == "" {
		return c.createCheckRun(ctx, request)
	}

	return request.StatusId, c.updateCheckRun(ctx, request, request.StatusId)
}

func (c *ChecksClientWrapper) createCheckRun(ctx context.Context, request types.UpdateStatusRequest) (string, error) {
	status, conclusion := c.resolveChecksStatus(request.State)
	createCheckRunOpts := github.CreateCheckRunOptions{
		Name:       request.StatusName,
		HeadSHA:    request.Ref,
		Status:     &status,
		DetailsURL: &request.DetailsURL,
		Output:     c.createCheckRunOutput(request),
	}

	// Conclusion is required if status is Completed
	if status == Completed.String() {
		createCheckRunOpts.Conclusion = &conclusion
	}

	return c.GithubClient.CreateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, createCheckRunOpts)
}

func (c *ChecksClientWrapper) updateCheckRun(ctx context.Context, request types.UpdateStatusRequest, checkRunId string) error {
	status, conclusion := c.resolveChecksStatus(request.State)
	updateCheckRunOpts := github.UpdateCheckRunOptions{
		Name:       request.StatusName,
		Status:     &status,
		DetailsURL: &request.DetailsURL,
		Output:     c.createCheckRunOutput(request),
	}

	// Conclusion is required if status is Completed
	if status == Completed.String() {
		updateCheckRunOpts.Conclusion = &conclusion
	}

	checkRunIdInt, err := strconv.ParseInt(checkRunId, 10, 64)
	if err != nil {
		return err
	}

	return c.GithubClient.UpdateCheckRun(ctx, request.Repo.Owner, request.Repo.Name, checkRunIdInt, updateCheckRunOpts)
}

func (c *ChecksClientWrapper) createCheckRunOutput(request types.UpdateStatusRequest) *github.CheckRunOutput {

	// Add Jobs URL if a project plan/apply command
	var summary string
	if strings.Contains(request.StatusName, ":") &&
		(strings.Contains(request.StatusName, "plan") || strings.Contains(request.StatusName, "apply")) {
		if request.DetailsURL != "" {
			summary = fmt.Sprintf("%s\n[Logs](%s)", request.Description, request.DetailsURL)
		}
	}

	checkRunOutput := github.CheckRunOutput{
		Title:   &request.StatusName,
		Summary: &summary,
	}

	if request.Output != "" {
		checkRunOutput.Text = c.capCheckRunOutput(request.Output)
	}

	return &checkRunOutput
}

// Cap the output string if it exceeds the max checks output length
func (c *ChecksClientWrapper) capCheckRunOutput(output string) *string {
	cappedOutput := output
	if len(output) > maxChecksOutputLength {
		cappedOutput = output[:maxChecksOutputLength]
	}
	return &cappedOutput
}

// Github Checks uses Status and Conclusion to report status of the check run. Need to map models.CommitStatus to Status and Conclusion
// Status -> queued, in_progress, completed
// Conclusion -> failure, neutral, cancelled, timed_out, or action_required. (Optional. Required if you provide a status of "completed".)
func (c *ChecksClientWrapper) resolveChecksStatus(state models.CommitStatus) (string, string) {
	status := Queued
	conclusion := Neutral

	switch state {
	case models.SuccessCommitStatus:
		status = Completed
		conclusion = Success

	case models.PendingCommitStatus:
		status = InProgress

	case models.FailedCommitStatus:
		status = Completed
		conclusion = Failure
	}

	return status.String(), conclusion.String()
}
