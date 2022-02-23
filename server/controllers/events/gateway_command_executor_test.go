package events

import (
	"errors"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	sns_mocks "github.com/runatlantis/atlantis/server/lyft/aws/sns/mocks"
	sns_matchers "github.com/runatlantis/atlantis/server/lyft/aws/sns/mocks/matchers"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestExecuteCommentCommand_Success(t *testing.T) {
	RegisterMockTestingT(t)
	mockCommandRunner := mocks.NewMockCommandRunner()
	mockWriter := sns_mocks.NewMockWriter()
	When(mockWriter.Write(sns_matchers.AnySliceOfByte())).ThenReturn(nil)
	executor := GatewayCommandExecutor{
		SNSWriter:     mockWriter,
		CommandRunner: mockCommandRunner,
	}
	req := createExampleRequest(t)
	resp := executor.ExecuteCommentCommand(req, models.Repo{}, nil, nil, models.User{}, 0, nil, time.Time{})
	mockWriter.VerifyWasCalledOnce().Write(sns_matchers.AnySliceOfByte())
	Assert(t, resp.err.code == 0, "response should have no error")
	Assert(t, resp.body == "Processing...", "response should be processing")
}

func TestExecuteCommentCommand_Failure(t *testing.T) {
	RegisterMockTestingT(t)
	mockCommandRunner := mocks.NewMockCommandRunner()
	mockWriter := sns_mocks.NewMockWriter()
	When(mockWriter.Write(sns_matchers.AnySliceOfByte())).ThenReturn(errors.New("marshal err"))
	executor := GatewayCommandExecutor{
		SNSWriter:     mockWriter,
		CommandRunner: mockCommandRunner,
	}
	req := createExampleRequest(t)
	resp := executor.ExecuteCommentCommand(req, models.Repo{}, nil, nil, models.User{}, 0, nil, time.Time{})
	mockWriter.VerifyWasCalledOnce().Write(sns_matchers.AnySliceOfByte())
	Assert(t, resp.err.code == 400, "response should have bad request error")
	Assert(t, resp.body == "Writing gateway message to sns topic: marshal err", "response should be a marshal err")
}

func TestExecuteAutoplanCommand_OpenWithTerraformChanges(t *testing.T) {
	RegisterMockTestingT(t)
	mockWriter := sns_mocks.NewMockWriter()
	When(mockWriter.Write(sns_matchers.AnySliceOfByte())).ThenReturn(nil)
	mockCommandRunner := mocks.NewMockCommandRunner()
	When(mockCommandRunner.RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())).ThenReturn(true)
	executor := GatewayCommandExecutor{
		SNSWriter:     mockWriter,
		CommandRunner: mockCommandRunner,
	}
	req := createExampleRequest(t)
	logger := logging.NewNoopLogger(t)

	resp := executor.ExecuteAutoplanCommand(req, models.OpenedPullEvent, models.Repo{}, models.Repo{}, models.PullRequest{}, models.User{}, time.Time{}, logger)
	mockWriter.VerifyWasCalledOnce().Write(sns_matchers.AnySliceOfByte())
	mockCommandRunner.VerifyWasCalledOnce().RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())
	Assert(t, resp.err.code == 0, "response should have no error")
	Assert(t, resp.body == "Processing...", "response should be processing")
}

func TestExecuteAutoplanCommand_OpenWithoutTerraformChanges(t *testing.T) {
	RegisterMockTestingT(t)
	mockWriter := sns_mocks.NewMockWriter()
	When(mockWriter.Write(sns_matchers.AnySliceOfByte())).ThenReturn(nil)
	mockCommandRunner := mocks.NewMockCommandRunner()
	When(mockCommandRunner.RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())).ThenReturn(false)
	executor := GatewayCommandExecutor{
		SNSWriter:     mockWriter,
		CommandRunner: mockCommandRunner,
	}
	req := createExampleRequest(t)
	logger := logging.NewNoopLogger(t)

	resp := executor.ExecuteAutoplanCommand(req, models.OpenedPullEvent, models.Repo{}, models.Repo{}, models.PullRequest{}, models.User{}, time.Time{}, logger)
	mockWriter.VerifyWasCalled(Never()).Write(sns_matchers.AnySliceOfByte())
	mockCommandRunner.VerifyWasCalledOnce().RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())
	Assert(t, resp.err.code == 0, "response should have no error")
	Assert(t, resp.body == "", "response should be empty")
}

func TestExecuteAutoplanCommand_Close(t *testing.T) {
	RegisterMockTestingT(t)
	mockWriter := sns_mocks.NewMockWriter()
	When(mockWriter.Write(sns_matchers.AnySliceOfByte())).ThenReturn(nil)
	mockCommandRunner := mocks.NewMockCommandRunner()
	When(mockCommandRunner.RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())).ThenReturn(false)
	executor := GatewayCommandExecutor{
		SNSWriter:     mockWriter,
		CommandRunner: mockCommandRunner,
	}
	req := createExampleRequest(t)
	logger := logging.NewNoopLogger(t)

	resp := executor.ExecuteAutoplanCommand(req, models.ClosedPullEvent, models.Repo{}, models.Repo{}, models.PullRequest{}, models.User{}, time.Time{}, logger)
	mockWriter.VerifyWasCalledOnce().Write(sns_matchers.AnySliceOfByte())
	mockCommandRunner.VerifyWasCalled(Never()).RunPseudoAutoplanCommand(
		matchers.AnyModelsRepo(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), matchers.AnyModelsUser())
	Assert(t, resp.err.code == 0, "response should have no error")
	Assert(t, resp.body == "", "response should be empty")
}

func createExampleRequest(t *testing.T) *http.Request {
	url, err := url.Parse("http://www.atlantis.com")
	assert.NoError(t, err)
	req := &http.Request{
		URL: url,
	}
	return req
}
