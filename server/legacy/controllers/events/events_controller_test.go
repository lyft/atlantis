// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/google/go-github/v45/github"
	. "github.com/petergtz/pegomock"
	events_controllers "github.com/runatlantis/atlantis/server/legacy/controllers/events"
	"github.com/runatlantis/atlantis/server/legacy/events"
	emocks "github.com/runatlantis/atlantis/server/legacy/events/mocks"
	vcsmocks "github.com/runatlantis/atlantis/server/legacy/events/vcs/mocks"
	httputils "github.com/runatlantis/atlantis/server/legacy/http"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/models"
	event_types "github.com/runatlantis/atlantis/server/neptune/gateway/event"
	. "github.com/runatlantis/atlantis/testing"
)

func AnyRepo() models.Repo {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf(models.Repo{})))
	return models.Repo{}
}

func AnyStatus() []*github.RepoStatus {
	RegisterMatcher(NewAnyMatcher(reflect.TypeOf(github.RepoStatus{})))
	return []*github.RepoStatus{}
}

func TestPost_NotGitlab(t *testing.T) {
	t.Log("when the request is not for gitlab or github a 400 is returned")
	e, _, _, _, _ := setup(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "", bytes.NewBuffer(nil))
	e.Post(w, req)
	ResponseContains(t, w, http.StatusBadRequest, "Ignoring request")
}

//nolint:unparam
func setup(t *testing.T) (events_controllers.VCSEventsController, *emocks.MockEventParsing, *emocks.MockPullCleaner, *vcsmocks.MockClient, *emocks.MockCommentParsing) {
	RegisterMockTestingT(t)
	p := emocks.NewMockEventParsing()
	cp := emocks.NewMockCommentParsing()
	c := emocks.NewMockPullCleaner()
	vcsmock := vcsmocks.NewMockClient()
	repoAllowlistChecker, err := events.NewRepoAllowlistChecker("*")
	Ok(t, err)
	ctxLogger := logging.NewNoopCtxLogger(t)
	scope, _, _ := metrics.NewLoggingScope(ctxLogger, "null")
	e := events_controllers.VCSEventsController{
		Logger:               ctxLogger,
		Scope:                scope,
		Parser:               p,
		CommentEventHandler:  noopCommentHandler{},
		PREventHandler:       noopPRHandler{},
		CommentParser:        cp,
		SupportedVCSHosts:    []models.VCSHostType{},
		RepoAllowlistChecker: repoAllowlistChecker,
		VCSClient:            vcsmock,
	}
	return e, p, c, vcsmock, cp
}

// This struct shouldn't be using these anyways since it should be broken down into separate packages (ie. see github)
// therefore we're just using noop implementations here for now.
// agreed this means we're not verifying any of the arguments passed in, but that's fine since we don't use any of these providers
// atm.
type noopPRHandler struct{}

func (h noopPRHandler) Handle(ctx context.Context, request *httputils.BufferedRequest, event event_types.PullRequest) error {
	return nil
}

type noopCommentHandler struct{}

func (h noopCommentHandler) Handle(ctx context.Context, request *httputils.BufferedRequest, event event_types.Comment) error {
	return nil
}
