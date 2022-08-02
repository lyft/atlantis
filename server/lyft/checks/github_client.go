package checks

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v31/github"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/db"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

// Reference: https://github.com/github/docs/issues/3765
const maxChecksOutputLength = 65535

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

// [WENGINES-4643] TODO: Remove this wrapper and add checks implementation to UpdateStatus() directly after github checks is stable
type ChecksClientWrapper struct {
	*vcs.GithubClient
	FeatureAllocator feature.Allocator
	Logger           logging.Logger
	Db               *db.BoltDB
}

func (c *ChecksClientWrapper) UpdateStatus(ctx context.Context, request types.UpdateStatusRequest) error {

	if !c.isChecksEnabled(ctx, request) {
		return c.GithubClient.UpdateStatus(ctx, request)
	}

	// Project level policy check command status is set to either success or failure directly
	// So, we can't use the Pending check to determine if this checkrun already exists
	// We attempt to retrieve the checkrun and if it exists, we update that check run which happens when a user approves failing policies
	// If not, we create a new one and put it into the db which happens when policy_check command is run for the first time
	if c.isProjectLevelPolicyCheckCommand(request.StatusName) {
		checkRun, err := c.Db.GetCheckRunForStatus(request.StatusName, request.Repo, request.PullNum)
		if err != nil {
			return errors.Wrapf(err, "getting checkrun Id from db for %s", request.StatusName)
		}

		// DNE in the db, so we create a new checkrun
		if checkRun == nil {
			return c.createCheckRun(ctx, request)
		}
		// Update the existing checkrun
		return c.updateCheckRun(ctx, *checkRun, request)

	}

	// Pending state when it's a new run. So, we create a new checkrun
	if request.State == models.PendingCommitStatus {
		return c.createCheckRun(ctx, request)
	}

	// Get checkrun from db and update the existing checkrun
	checkRun, err := c.Db.GetCheckRunForStatus(request.StatusName, request.Repo, request.PullNum)
	if err != nil {
		return errors.Wrapf(err, "getting checkrun Id from db for %s", request.StatusName)
	}

	// This is likely a bug
	if checkRun == nil {
		return errors.New("checkrun dne in db")
	}

	return c.updateCheckRun(ctx, *checkRun, request)
}

func (c *ChecksClientWrapper) isProjectLevelPolicyCheckCommand(statusName string) bool {
	return strings.Contains(statusName, "policy_check") && strings.Contains(statusName, ":")
}

func (c *ChecksClientWrapper) createCheckRun(ctx context.Context, request types.UpdateStatusRequest) error {
	checkRun, err := c.GithubClient.CreateCheckStatus(ctx, request.Repo, c.populateCreateCheckRunOptions(request))
	if err != nil {
		return errors.Wrapf(err, "creating checkrun for %s", request.StatusName)
	}

	// Get output from checkrun and store in db, used to populate next status updates since github does not store its state
	var output string
	if checkRun.Output != nil && checkRun.Output.Text != nil {
		output = *checkRun.Output.Text
	}

	// Store the checkrun ID in boltdb
	if err = c.Db.UpdateCheckRunForStatus(request.StatusName, request.Repo, request.PullNum, models.CheckRunStatus{
		ID:      strconv.FormatInt(*checkRun.ID, 10),
		Output:  output,
		JobsURL: *checkRun.DetailsURL,
	}); err != nil {
		return errors.Wrapf(err, "updating checkrun id in db for %s", request.StatusName)
	}
	return nil
}

func (c *ChecksClientWrapper) updateCheckRun(ctx context.Context, checkRun models.CheckRunStatus, request types.UpdateStatusRequest) error {
	checkRunIdInt, err := strconv.ParseInt(checkRun.ID, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "parsing checkrunId for %s", request.StatusName)
	}

	// Populate summary and output if not already present
	if request.Output == "" {
		request.Output = checkRun.Output
	}

	return c.GithubClient.UpdateCheckStatus(ctx, request.Repo, checkRunIdInt, c.populateUpdateCheckRunOptions(request, checkRun))
}

func (c *ChecksClientWrapper) isChecksEnabled(ctx context.Context, request types.UpdateStatusRequest) bool {
	shouldAllocate, err := c.FeatureAllocator.ShouldAllocate(feature.GithubChecks, feature.FeatureContext{
		RepoName:         request.Repo.FullName,
		PullCreationTime: request.PullCreationTime,
	})
	if err != nil {
		c.Logger.ErrorContext(ctx, fmt.Sprintf("unable to allocate for feature: %s", feature.GithubChecks), map[string]interface{}{
			"error": err.Error(),
		})
	}

	return shouldAllocate
}

func (c *ChecksClientWrapper) populateCreateCheckRunOptions(request types.UpdateStatusRequest) github.CreateCheckRunOptions {
	status, conclusion := c.resolveChecksStatus(request.State)
	output := c.capCheckRunOutput(request.Output)
	summary := c.summaryWithJobURL(request.StatusName, request.Description, request.DetailsURL)

	checkRunOutput := &github.CheckRunOutput{
		Title:   &request.StatusName,
		Summary: &summary,
	}

	// Only add text if output is not empty to avoid an empty output box in the checkrun UI
	if output != "" {
		checkRunOutput.Text = &output
	}

	createCheckRunOptions := github.CreateCheckRunOptions{
		Name:    request.StatusName,
		HeadSHA: request.Ref,
		Status:  &status,
		Output:  checkRunOutput,
	}

	// Conclusion is required if status is Completed
	if status == Completed.String() {
		createCheckRunOptions.Conclusion = &conclusion
	}

	return createCheckRunOptions
}

func (c *ChecksClientWrapper) populateUpdateCheckRunOptions(request types.UpdateStatusRequest, checkRunStatus models.CheckRunStatus) github.UpdateCheckRunOptions {
	// Populate output from previous status update if request.Output is empty
	if request.Output == "" {
		request.Output = checkRunStatus.Output
	}

	// Populate detailsURL from the previous status update if request.DetailsURL is empty
	if request.DetailsURL == "" {
		request.DetailsURL = checkRunStatus.JobsURL
	}

	status, conclusion := c.resolveChecksStatus(request.State)
	output := c.capCheckRunOutput(request.Output)
	summary := c.summaryWithJobURL(request.StatusName, request.Description, checkRunStatus.JobsURL)

	updateCheckRunOptions := github.UpdateCheckRunOptions{
		Name:    request.StatusName,
		HeadSHA: &request.Ref,
		Status:  &status,
		Output: &github.CheckRunOutput{
			Title:   &request.StatusName,
			Summary: &summary,
			Text:    &output,
		},
	}

	// Conclusion is required if status is Completed
	if status == Completed.String() {
		updateCheckRunOptions.Conclusion = &conclusion
	}

	return updateCheckRunOptions
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

// Cap the output string if it exceeds the max checks output length
func (c *ChecksClientWrapper) capCheckRunOutput(output string) string {
	if len(output) > maxChecksOutputLength {
		return output[:maxChecksOutputLength]
	}
	return output
}

// Append job URL to summary if it's a project plan or apply operation bc we currently only stream logs for these two operations
func (g *ChecksClientWrapper) summaryWithJobURL(statusName string, summary string, jobsURL string) string {
	if strings.Contains(statusName, ":") &&
		(strings.Contains(statusName, "plan") || strings.Contains(statusName, "apply")) {

		if jobsURL != "" {
			return fmt.Sprintf("%s\n[Logs](%s)", summary, jobsURL)
		}
	}
	return summary
}
