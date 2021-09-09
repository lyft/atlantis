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
	"errors"
	"testing"

	"github.com/runatlantis/atlantis/server/events/db"
	"github.com/runatlantis/atlantis/server/logging"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	lockmocks "github.com/runatlantis/atlantis/server/events/locking/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	vcsmocks "github.com/runatlantis/atlantis/server/events/vcs/mocks"
	. "github.com/runatlantis/atlantis/testing"
)

func TestCleanUpPullWorkspaceErr(t *testing.T) {
	t.Log("when workspace.Delete returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkingDir()
	pce := events.PullClosedExecutor{
		WorkingDir:         w,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
	}
	err := errors.New("err")
	When(w.GetWorkingDir(fixtures.GithubRepo, fixtures.Pull, "default")).ThenReturn("", err)
	When(w.Delete(fixtures.GithubRepo, fixtures.Pull)).ThenReturn(err)
	actualErr := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Equals(t, "cleaning workspace: err", actualErr.Error())
}

func TestCleanUpPullUnlockErr(t *testing.T) {
	t.Log("when locker.UnlockByPull returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkingDir()
	l := lockmocks.NewMockLocker()
	pce := events.PullClosedExecutor{
		Locker:             l,
		WorkingDir:         w,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
	}
	err := errors.New("err")
	When(w.GetWorkingDir(fixtures.GithubRepo, fixtures.Pull, "default")).ThenReturn("", err)
	When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(nil, err)
	actualErr := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Equals(t, "cleaning up locks: err", actualErr.Error())
}

func TestCleanUpPullNoLocks(t *testing.T) {
	t.Log("when there are no locks to clean up, we don't comment")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkingDir()
	l := lockmocks.NewMockLocker()
	cp := vcsmocks.NewMockClient()
	tmp, cleanup := TempDir(t)
	defer cleanup()
	db, err := db.New(tmp)
	Ok(t, err)
	pce := events.PullClosedExecutor{
		Locker:             l,
		VCSClient:          cp,
		WorkingDir:         w,
		DB:                 db,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
	}
	err = errors.New("err")
	When(w.GetWorkingDir(fixtures.GithubRepo, fixtures.Pull, "default")).ThenReturn("", err)
	When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(nil, nil)
	err = pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Ok(t, err)
	cp.VerifyWasCalled(Never()).CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString(), AnyString())
}

func TestCleanUpPullComments(t *testing.T) {
	t.Log("should comment correctly")
	RegisterMockTestingT(t)
	cases := []struct {
		Description string
		Locks       []models.ProjectLock
		Exp         string
	}{
		{
			"single lock, empty path",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", ""),
					Workspace: "default",
				},
			},
			"- dir: `.` workspace: `default`",
		},
		{
			"single lock, non-empty path",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "default",
				},
			},
			"- dir: `path` workspace: `default`",
		},
		{
			"single path, multiple workspaces",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace2",
				},
			},
			"- dir: `path` workspaces: `workspace1`, `workspace2`",
		},
		{
			"multiple paths, multiple workspaces",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace2",
				},
				{
					Project:   models.NewProject("owner/repo", "path2"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path2"),
					Workspace: "workspace2",
				},
			},
			"- dir: `path` workspaces: `workspace1`, `workspace2`\n- dir: `path2` workspaces: `workspace1`, `workspace2`",
		},
	}
	for _, c := range cases {
		func() {
			w := mocks.NewMockWorkingDir()
			cp := vcsmocks.NewMockClient()
			l := lockmocks.NewMockLocker()
			tmp, cleanup := TempDir(t)
			defer cleanup()
			db, err := db.New(tmp)
			Ok(t, err)
			pce := events.PullClosedExecutor{
				Locker:             l,
				VCSClient:          cp,
				WorkingDir:         w,
				DB:                 db,
				PullClosedTemplate: &events.PullClosedEventTemplate{},
				Logger:             logging.NewNoopLogger(t),
			}
			t.Log("testing: " + c.Description)
			err = errors.New("err")
			When(w.GetWorkingDir(fixtures.GithubRepo, fixtures.Pull, "default")).ThenReturn("", err)
			When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(c.Locks, nil)
			err = pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
			Ok(t, err)
			_, _, comment, _ := cp.VerifyWasCalledOnce().CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString(), AnyString()).GetCapturedArguments()

			expected := "Locks and plans deleted for the projects and workspaces modified in this pull request:\n\n" + c.Exp
			Equals(t, expected, comment)
		}()
	}
}

/*
Testing Resource cleanup

1. Add a project to buffers manually and run the cleanup.

*/

// func TestCleanUpLogStreaming(t *testing.T) {
// 	w := mocks.NewMockWorkingDir()
// 	cp := vcsmocks.NewMockClient()
// 	l := lockmocks.NewMockLocker()
// 	logger := logging.NewNoopLogger(t)
// 	parserValidator := yamlmocks.NewMockIParserValidator()
// 	prjCmdOutput := make(chan *models.ProjectCmdOutputLine)
// 	prjCmdOutHandler := handlers.NewProjectCommandOutputHandler(prjCmdOutput, logger)
// 	ctx := models.ProjectCommandContext{
// 		BaseRepo:    fixtures.GithubRepo,
// 		Pull:        fixtures.Pull,
// 		ProjectName: *fixtures.Project.Name,
// 	}

// 	// Go routine to add new
// 	go prjCmdOutHandler.Handle()

// 	tmp, cleanup := TempDir(t)
// 	defer cleanup()
// 	db, err := db.New(tmp)
// 	Ok(t, err)

// 	pullClosedExecutor := events.PullClosedExecutor{
// 		Locker:                   l,
// 		VCSClient:                cp,
// 		WorkingDir:               w,
// 		DB:                       db,
// 		PullClosedTemplate:       &events.PullClosedEventTemplate{},
// 		LogStreamResourceCleaner: prjCmdOutHandler,
// 		Logger:                   logging.NewNoopLogger(t),
// 		ParserVarlidator:         parserValidator,
// 	}

// 	// Send a tf message to log-streaming handler
// 	prjCmdOutHandler.Send(ctx, "Test Message")

// 	// Make sure channels are added.
// 	time.Sleep(1 * time.Second)

// 	// Clean up.
// 	err = pullClosedExecutor.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
// 	Ok(t, err)

// 	repoDir := "/Users/TestEnv/runatlantis/atlantis/1/default"
// 	// Mock workingDir to return: /Users/TestEnv/runatlantis/atlantis/1/default
// 	When(w.GetWorkingDir(fixtures.GithubRepo, fixtures.Pull, "default")).ThenReturn(repoDir, nil)

// 	// Mock VCSClient to return: main.tf
// 	When(cp.GetModifiedFiles(fixtures.GithubRepo, fixtures.Pull)).ThenReturn([]string{"main.tf"}, nil)

// 	When(parserValidator.HasRepoCfg(repoDir)).ThenReturn(true, nil)
// 	When(parserValidator.ParseRepoCfg(repoDir, Any))

// }
