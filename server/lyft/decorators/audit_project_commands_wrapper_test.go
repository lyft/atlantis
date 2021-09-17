package decorators_test

import (
	"encoding/json"
	"errors"
	"testing"

	. "github.com/runatlantis/atlantis/testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"github.com/runatlantis/atlantis/server/logging"
	snsMocks "github.com/runatlantis/atlantis/server/lyft/aws/sns/mocks"
	"github.com/runatlantis/atlantis/server/lyft/decorators"
)

func TestAuditProjectCommandsWrapper(t *testing.T) {
	RegisterMockTestingT(t)

	cases := []struct {
		Description string
		Success     bool
		Failure     bool
		Error       bool
	}{
		{
			Description: "apply success",
			Success:     true,
		},
		{
			Description: "apply error",
			Error:       true,
		},
		{
			Description: "apply failure",
			Failure:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			snsMock := snsMocks.NewMockWriter()
			projectCmdRunnerMock := mocks.NewMockProjectCommandRunner()
			auditPrjCmds := &decorators.AuditProjectCommandWrapper{
				SnsWriter:            snsMock,
				ProjectCommandRunner: projectCmdRunnerMock,
			}

			prjRslt := models.ProjectResult{}

			if c.Error {
				prjRslt.Error = errors.New("oh-no")
			}

			if c.Failure {
				prjRslt.Failure = "oh-no"
			}

			ctx := models.ProjectCommandContext{
				Log:         logging.NewNoopLogger(t),
				Steps:       []valid.Step{},
				ProjectName: "test-project",
				User: models.User{
					Username: "test-user",
				},
				Workspace: "default",
				PullReqStatus: models.PullReqStatus{
					Approved: false,
				},
				RepoRelDir: ".",
				Tags: map[string]string{
					"environment":  "production",
					"service_name": "test-service",
				},
			}

			When(snsMock.Write(matchers.AnySliceOfByte())).ThenReturn(nil)
			When(projectCmdRunnerMock.Apply(matchers.AnyModelsProjectCommandContext())).ThenReturn(prjRslt)

			auditPrjCmds.Apply(ctx)

			eventBefore := &decorators.ApplyEvent{}
			eventAfter := &decorators.ApplyEvent{}
			eventPayload := snsMock.VerifyWasCalled(Twice()).Write(matchers.AnySliceOfByte()).GetAllCapturedArguments()

			json.Unmarshal(eventPayload[0], eventBefore)
			json.Unmarshal(eventPayload[1], eventAfter)

			Equals(t, eventBefore.State, decorators.ApplyEventInitiated)
			Equals(t, eventBefore.EndTime, "")
			Equals(t, eventBefore.RootName, "test-project")
			Equals(t, eventBefore.Environment, "production")
			Equals(t, eventBefore.InitiatingUser, "test-user")
			Equals(t, eventBefore.Project, "test-service")
			Assert(t, eventBefore.StartTime != "", "start time must be set")

			if c.Success {
				Equals(t, eventAfter.State, decorators.ApplyEventSuccess)
			} else {
				Equals(t, eventAfter.State, decorators.ApplyEventError)
			}

			Assert(t, eventBefore.StartTime == eventAfter.StartTime, "start time should not change")
			Assert(t, eventAfter.EndTime != "", "end time must be set")
			Assert(t, eventBefore.ID == eventAfter.ID, "id should not change")
		})
	}
}
