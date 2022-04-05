package events_test

import (
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestProjectCommandContextBuilder_PullStatus(t *testing.T) {

	scope := tally.NewTestScope("test", nil)
	mockCommentBuilder := mocks.NewMockCommentBuilder()
	subject := events.NewProjectCommandContextBuilder(mockCommentBuilder)

	projRepoRelDir := "dir1"
	projWorkspace := "default"
	projName := "project1"

	projCfg := valid.MergedProjectCfg{
		RepoRelDir: projRepoRelDir,
		Workspace:  projWorkspace,
		Name:       projName,
		Workflow: valid.Workflow{
			Name:  valid.DefaultWorkflowName,
			Apply: valid.DefaultApplyStage,
		},
	}

	pullStatus := &models.PullStatus{
		Projects: []models.ProjectStatus{},
	}

	commandCtx := &command.Context{
		Log:        logging.NewNoopLogger(t),
		PullStatus: pullStatus,
		Scope:      scope,
	}

	expectedApplyCmt := "Apply Comment"
	expectedPlanCmt := "Plan Comment"

	t.Run("with project name defined", func(t *testing.T) {
		When(mockCommentBuilder.BuildPlanComment(projRepoRelDir, projWorkspace, projName, []string{})).ThenReturn(expectedPlanCmt)
		When(mockCommentBuilder.BuildApplyComment(projRepoRelDir, projWorkspace, projName, false)).ThenReturn(expectedApplyCmt)

		pullStatus.Projects = []models.ProjectStatus{
			{
				Status:      models.ErroredPolicyCheckStatus,
				ProjectName: "project1",
				RepoRelDir:  "dir1",
			},
		}
		contextFlags := &command.ContextFlags{
			DeleteSourceBranchOnMerge: false,
			ParallelApply:             false,
			ParallelPlan:              false,
			Verbose:                   false,
			ForceApply:                false,
		}
		result := subject.BuildProjectContext(commandCtx, command.Plan, projCfg, []string{}, "some/dir", contextFlags)

		assert.Equal(t, models.ErroredPolicyCheckStatus, result[0].ProjectPlanStatus)
	})

	t.Run("when ParallelApply is set to true", func(t *testing.T) {
		projCfg.Name = "Apply Comment"
		When(mockCommentBuilder.BuildPlanComment(projRepoRelDir, projWorkspace, "", []string{})).ThenReturn(expectedPlanCmt)
		When(mockCommentBuilder.BuildApplyComment(projRepoRelDir, projWorkspace, "", false)).ThenReturn(expectedApplyCmt)
		pullStatus.Projects = []models.ProjectStatus{
			{
				Status:     models.ErroredPlanStatus,
				RepoRelDir: "dir2",
			},
			{
				Status:     models.ErroredPolicyCheckStatus,
				RepoRelDir: "dir1",
			},
		}
		contextFlags := &command.ContextFlags{
			DeleteSourceBranchOnMerge: false,
			ParallelApply:             true,
			ParallelPlan:              false,
			Verbose:                   false,
			ForceApply:                false,
		}
		result := subject.BuildProjectContext(commandCtx, command.Plan, projCfg, []string{}, "some/dir", contextFlags)

		assert.True(t, result[0].ParallelApplyEnabled)
		assert.False(t, result[0].ParallelPlanEnabled)
	})
}
