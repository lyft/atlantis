package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/runatlantis/atlantis/server/controllers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/handlers/mocks"
	"github.com/runatlantis/atlantis/server/handlers/mocks/matchers"
)

func TestGetLogStream_WebSockets(t *testing.T) {
	t.Run("Should Group by Project Info", func(t *testing.T) {
		RegisterMockTestingT(t)
		websocketMock := mocks.NewMockWebsocketHandler()
		projectOutputHandler := mocks.NewMockProjectCommandOutputHandler()
		logger := logging.NewNoopLogger(t)
		websocketWriterMock := mocks.NewMockWebsocketResponseWriter()
		params := map[string]string{
			"org":     "test-org",
			"repo":    "test-repo",
			"pull":    "1",
			"project": "test-project",
		}
		request, _ := http.NewRequest(http.MethodGet, "/logStreaming/org/repo/1/project/ws", nil)
		request = mux.SetURLVars(request, params)
		response := httptest.NewRecorder()
		logStreamingController := &controllers.LogStreamingController{
			Logger:                      logger,
			WebsocketHandler:            websocketMock,
			ProjectCommandOutputHandler: projectOutputHandler,
		}
		tempRepo := models.Repo{
			FullName: "test-repo",
		}
		tempPullNum := models.PullRequest{
			Num: 1,
		}

		ctx := models.ProjectCommandContext{
			BaseRepo:    tempRepo,
			Pull:        tempPullNum,
			ProjectName: "test-project",
		}

		When(websocketMock.Upgrade(matchers.AnyHttpResponseWriter(), matchers.AnyPtrToHttpRequest(), matchers.AnyHttpHeader())).ThenReturn(websocketWriterMock, nil)

		projectOutputHandler.Send(ctx, "Test Terraform Output")
		projectOutputHandler.Handle()
		logStreamingController.GetLogStreamWS(response, request)

		websocketWriterMock.VerifyWasCalled(Once())
	})
}
