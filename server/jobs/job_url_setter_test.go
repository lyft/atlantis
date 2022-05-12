package jobs_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/jobs"
	"github.com/runatlantis/atlantis/server/jobs/mocks"
	"github.com/runatlantis/atlantis/server/jobs/mocks/matchers"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

func TestJobURLSetter(t *testing.T) {
	ctx := createTestProjectCmdContext(t)

	t.Run("update project status with project jobs url", func(t *testing.T) {
		RegisterMockTestingT(t)
		projectStatusUpdater := &mockProjectStatusUpdater{}
		projectJobURLGenerator := mocks.NewMockProjectJobURLGenerator()
		url := "url-to-project-jobs"
		jobURLSetter := jobs.NewJobURLSetter(projectJobURLGenerator, projectStatusUpdater)

		When(projectJobURLGenerator.GenerateProjectJobURL(matchers.EqModelsProjectCommandContext(ctx))).ThenReturn(url, nil)
		_, err := jobURLSetter.SetJobURLWithStatus(ctx, command.Plan, models.PendingCommitStatus, "")
		Ok(t, err)

		assert.Equal(t, ctx, projectStatusUpdater.CalledPrjCtx)
		assert.Equal(t, command.Plan, projectStatusUpdater.CalledCmdName)
		assert.Equal(t, models.PendingCommitStatus, projectStatusUpdater.CalledStatus)
		assert.Equal(t, url, projectStatusUpdater.CalledUrl)

	})

	t.Run("update project status with project jobs url error", func(t *testing.T) {
		RegisterMockTestingT(t)
		projectStatusUpdater := &mockProjectStatusUpdater{}
		projectJobURLGenerator := mocks.NewMockProjectJobURLGenerator()
		jobURLSetter := jobs.NewJobURLSetter(projectJobURLGenerator, projectStatusUpdater)

		When(projectJobURLGenerator.GenerateProjectJobURL(matchers.EqModelsProjectCommandContext(ctx))).ThenReturn("url-to-project-jobs", errors.New("some error"))
		_, err := jobURLSetter.SetJobURLWithStatus(ctx, command.Plan, models.PendingCommitStatus, "")
		assert.Error(t, err)
	})
}

type mockProjectStatusUpdater struct {
	CalledCtx      context.Context
	CalledPrjCtx   command.ProjectContext
	CalledCmdName  fmt.Stringer
	CalledStatus   models.CommitStatus
	CalledUrl      string
	CalledStatusId string
}

func (t *mockProjectStatusUpdater) UpdateProject(ctx context.Context, projectCtx command.ProjectContext, cmdName fmt.Stringer, status models.CommitStatus, url string, statusId string) (string, error) {
	t.CalledCtx = ctx
	t.CalledPrjCtx = projectCtx
	t.CalledCmdName = cmdName
	t.CalledStatus = status
	t.CalledUrl = url
	t.CalledStatusId = statusId
	return "", nil
}
