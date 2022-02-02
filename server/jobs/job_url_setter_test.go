package jobs

import (
	"errors"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/jobs/mocks"
	"github.com/runatlantis/atlantis/server/jobs/mocks/matchers"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

// update project status with project jobs url

func createTestProjectCmdContext(t *testing.T) models.ProjectCommandContext {
	logger := logging.NewNoopLogger(t)
	return models.ProjectCommandContext{
		BaseRepo: models.Repo{
			Name:  "test-repo",
			Owner: "test-org",
		},
		HeadRepo: models.Repo{
			Name:  "test-repo",
			Owner: "test-org",
		},
		Pull: models.PullRequest{
			Num:        1,
			HeadBranch: "master",
			BaseBranch: "master",
			Author:     "test-user",
			HeadCommit: "234r232432",
		},
		User: models.User{
			Username: "test-user",
		},
		Log:         logger,
		Workspace:   "myworkspace",
		RepoRelDir:  "test-dir",
		ProjectName: "test-project",
		JobID:       "1234",
	}
}

func TestJobURLSetter(t *testing.T) {
	ctx := createTestProjectCmdContext(t)

	t.Run("update project status with project jobs url", func(t *testing.T) {
		RegisterMockTestingT(t)
		projectStatusUpdater := mocks.NewMockProjectStatusUpdater()
		projectJobURLGenerator := mocks.NewMockProjectJobURLGenerator()
		url := "url-to-project-jobs"
		jobURLSetter := NewJobURLSetter(projectJobURLGenerator, projectStatusUpdater)

		When(projectJobURLGenerator.GenerateProjectJobURL(matchers.EqModelsProjectCommandContext(ctx))).ThenReturn(url, nil)
		When(projectStatusUpdater.UpdateProject(ctx, models.PlanCommand, models.PendingCommitStatus, url)).ThenReturn(nil)
		err := jobURLSetter.SetJobURLWithStatus(ctx, models.PlanCommand, models.PendingCommitStatus)
		Ok(t, err)

		projectStatusUpdater.VerifyWasCalledOnce().UpdateProject(ctx, models.PlanCommand, models.PendingCommitStatus, "url-to-project-jobs")
	})

	t.Run("update project status with project jobs url error", func(t *testing.T) {
		RegisterMockTestingT(t)
		projectStatusUpdater := mocks.NewMockProjectStatusUpdater()
		projectJobURLGenerator := mocks.NewMockProjectJobURLGenerator()
		jobURLSetter := NewJobURLSetter(projectJobURLGenerator, projectStatusUpdater)

		When(projectJobURLGenerator.GenerateProjectJobURL(matchers.EqModelsProjectCommandContext(ctx))).ThenReturn("url-to-project-jobs", errors.New("some error"))
		err := jobURLSetter.SetJobURLWithStatus(ctx, models.PlanCommand, models.PendingCommitStatus)
		assert.Error(t, err)
	})
}
