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

package server_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/logging"

	"github.com/gorilla/mux"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/controllers/templates"
	tMocks "github.com/runatlantis/atlantis/server/controllers/templates/mocks"
	"github.com/runatlantis/atlantis/server/core/locking/mocks"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

func TestIndex_LockErrorf(t *testing.T) {
	t.Log("index should return a 503 if unable to list locks")
	RegisterMockTestingT(t)
	l := mocks.NewMockLocker()
	When(l.List()).ThenReturn(nil, errors.New("err"))
	s := server.Server{
		Locker: l,
	}
	req, _ := http.NewRequest(http.MethodGet, "", bytes.NewBuffer(nil))
	w := httptest.NewRecorder()
	s.Index(w, req)
	ResponseContains(t, w, 503, "Could not retrieve locks: err")
}

func TestIndex_Success(t *testing.T) {
	t.Log("Index should render the index template successfully.")
	RegisterMockTestingT(t)
	l := mocks.NewMockLocker()
	al := mocks.NewMockApplyLocker()
	// These are the locks that we expect to be rendered.
	now := time.Now()
	locks := map[string]models.ProjectLock{
		"lkysow/atlantis-example/./default": {
			Pull: models.PullRequest{
				Num: 9,
			},
			Project: models.Project{
				RepoFullName: "lkysow/atlantis-example",
			},
			Time: now,
		},
	}
	When(l.List()).ThenReturn(locks, nil)
	it := tMocks.NewMockTemplateWriter()
	r := mux.NewRouter()
	atlantisVersion := "0.3.1"
	// Need to create a lock route since the server expects this route to exist.
	r.NewRoute().Path("/lock").
		Queries("id", "{id}").Name(server.LockViewRouteName)
	u, err := url.Parse("https://example.com")
	Ok(t, err)
	s := server.Server{
		Locker:          l,
		ApplyLocker:     al,
		IndexTemplate:   it,
		Router:          r,
		AtlantisVersion: atlantisVersion,
		AtlantisURL:     u,
		CtxLogger:       logging.NewNoopCtxLogger(t),
	}
	req, _ := http.NewRequest(http.MethodGet, "", bytes.NewBuffer(nil))
	w := httptest.NewRecorder()
	s.Index(w, req)
	it.VerifyWasCalledOnce().Execute(w, templates.IndexData{
		ApplyLock: templates.ApplyLockData{
			Locked:        false,
			Time:          time.Time{},
			TimeFormatted: "01-01-0001 00:00:00",
		},
		Locks: []templates.LockIndexData{
			{
				LockPath:      "/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
				RepoFullName:  "lkysow/atlantis-example",
				PullNum:       9,
				Time:          now,
				TimeFormatted: now.Format("02-01-2006 15:04:05"),
			},
		},
		AtlantisVersion: atlantisVersion,
	})
	ResponseContains(t, w, http.StatusOK, "")
}

func TestHealthz(t *testing.T) {
	s := server.Server{}
	req, _ := http.NewRequest(http.MethodGet, "/healthz", bytes.NewBuffer(nil))
	w := httptest.NewRecorder()
	s.Healthz(w, req)
	Equals(t, http.StatusOK, w.Result().StatusCode)
	body, _ := io.ReadAll(w.Result().Body)
	Equals(t, "application/json", w.Result().Header["Content-Type"][0])
	Equals(t,
		`{
  "status": "ok"
}`, string(body))
}
