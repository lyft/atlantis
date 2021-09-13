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
	"github.com/runatlantis/atlantis/server/handlers"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/stretchr/testify/assert"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	lockmocks "github.com/runatlantis/atlantis/server/events/locking/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	vcsmocks "github.com/runatlantis/atlantis/server/events/vcs/mocks"
	loggermocks "github.com/runatlantis/atlantis/server/logging/mocks"
	. "github.com/runatlantis/atlantis/testing"
)

func TestCleanUpPullWorkspaceErr(t *testing.T) {
	t.Log("when workspace.Delete returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkingDir()
	projectFinder := mocks.NewMockProjectFinder()
	pce := events.PullClosedExecutor{
		WorkingDir:         w,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
		ProjectFinder:      projectFinder,
	}
	err := errors.New("err")
	When(w.Delete(fixtures.GithubRepo, fixtures.Pull)).ThenReturn(err)
	actualErr := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Equals(t, "cleaning workspace: err", actualErr.Error())
}

func TestCleanUpPullUnlockErr(t *testing.T) {
	t.Log("when locker.UnlockByPull returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockWorkingDir()
	l := lockmocks.NewMockLocker()
	projectFinder := mocks.NewMockProjectFinder()
	pce := events.PullClosedExecutor{
		Locker:             l,
		WorkingDir:         w,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
		ProjectFinder:      projectFinder,
	}
	err := errors.New("err")
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
	projectFinder := mocks.NewMockProjectFinder()
	tmp, cleanup := TempDir(t)
	defer cleanup()
	db, err := db.New(tmp)
	Ok(t, err)
	pce := events.PullClosedExecutor{
		Locker:             l,
		WorkingDir:         w,
		DB:                 db,
		ProjectFinder:      projectFinder,
		PullClosedTemplate: &events.PullClosedEventTemplate{},
		Logger:             logging.NewNoopLogger(t),
	}
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
			cp := vcsmocks.NewMockClient()
			l := lockmocks.NewMockLocker()
			w := mocks.NewMockWorkingDir()
			projectFinder := mocks.NewMockProjectFinder()
			tmp, cleanup := TempDir(t)
			defer cleanup()
			db, err := db.New(tmp)
			Ok(t, err)
			pce := events.PullClosedExecutor{
				Locker:             l,
				DB:                 db,
				VCSClient:          cp,
				WorkingDir:         w,
				PullClosedTemplate: &events.PullClosedEventTemplate{},
				Logger:             logging.NewNoopLogger(t),
				ProjectFinder:      projectFinder,
			}
			t.Log("testing: " + c.Description)
			When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(c.Locks, nil)
			err = pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
			Ok(t, err)
			_, _, comment, _ := cp.VerifyWasCalledOnce().CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString(), AnyString()).GetCapturedArguments()

			expected := "Locks and plans deleted for the projects and workspaces modified in this pull request:\n\n" + c.Exp
			Equals(t, expected, comment)
		}()
	}
}

func TestCleanUpLogStreaming(t *testing.T) {
	RegisterMockTestingT(t)

	t.Run("Should Clean Up Log Streaming Resources When PR is closed", func(t *testing.T) {
		workingDir := mocks.NewMockWorkingDir()
		locker := lockmocks.NewMockLocker()
		client := vcsmocks.NewMockClient()
		projectFinder := mocks.NewMockProjectFinder()
		logger := logging.NewNoopLogger(t)

		// Log streaming resources
		prjCmdOutput := make(chan *models.ProjectCmdOutputLine)
		prjCmdOutHandler := handlers.NewProjectCommandOutputHandler(prjCmdOutput, logger)
		ctx := models.ProjectCommandContext{
			BaseRepo:    fixtures.GithubRepo,
			Pull:        fixtures.Pull,
			ProjectName: *fixtures.Project.Name,
		}

		go prjCmdOutHandler.Handle()
		prjCmdOutHandler.Send(ctx, "Test Message")

		tmp, cleanup := TempDir(t)
		defer cleanup()
		db, err := db.New(tmp)
		Ok(t, err)

		pullClosedExecutor := events.PullClosedExecutor{
			Locker:                   locker,
			WorkingDir:               workingDir,
			DB:                       db,
			VCSClient:                client,
			PullClosedTemplate:       &events.PullClosedEventTemplate{},
			LogStreamResourceCleaner: prjCmdOutHandler,
			Logger:                   logger,
			ProjectFinder:            projectFinder,
		}

		locks := []models.ProjectLock{
			{
				Project:   models.NewProject(fixtures.GithubRepo.FullName, ""),
				Workspace: "default",
			},
		}
		When(projectFinder.FindMatchingProjects(logger, fixtures.GithubRepo, fixtures.Pull)).ThenReturn([]string{ctx.PullInfo()}, nil)
		When(locker.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(locks, nil)

		// Clean up.
		err = pullClosedExecutor.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
		Ok(t, err)

		close(prjCmdOutput)
		_, _, comment, _ := client.VerifyWasCalledOnce().CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString(), AnyString()).GetCapturedArguments()
		expectedComment := "Locks and plans deleted for the projects and workspaces modified in this pull request:\n\n" + "- dir: `.` workspace: `default`"
		Equals(t, expectedComment, comment)

		dfPrjCmdOutputHandler := prjCmdOutHandler.(*handlers.DefaultProjectCommandOutputHandler)
		assert.Empty(t, dfPrjCmdOutputHandler.GetProjectOutputBuffer(ctx.PullInfo()))
		assert.Empty(t, dfPrjCmdOutputHandler.GetReceiverBufferForPull(ctx.PullInfo()))
	})

	t.Run("Should Log Error and continue clean up when FindMatchingProjects Fails", func(t *testing.T) {
		workingDir := mocks.NewMockWorkingDir()
		locker := lockmocks.NewMockLocker()
		client := vcsmocks.NewMockClient()
		projectFinder := mocks.NewMockProjectFinder()
		logger := loggermocks.NewMockSimpleLogging()

		tmp, cleanup := TempDir(t)
		defer cleanup()
		db, err := db.New(tmp)
		Ok(t, err)

		pullClosedExecutor := events.PullClosedExecutor{
			Locker:             locker,
			WorkingDir:         workingDir,
			DB:                 db,
			VCSClient:          client,
			PullClosedTemplate: &events.PullClosedEventTemplate{},
			Logger:             logger,
			ProjectFinder:      projectFinder,
		}

		locks := []models.ProjectLock{
			{
				Project:   models.NewProject(fixtures.GithubRepo.FullName, ""),
				Workspace: "default",
			},
		}
		expetedErr := errors.New("error finding projects")
		When(projectFinder.FindMatchingProjects(logger, fixtures.GithubRepo, fixtures.Pull)).ThenReturn([]string{}, expetedErr)
		When(locker.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(locks, nil)

		// Clean up.
		err = pullClosedExecutor.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
		Ok(t, err)

		_, _, comment, _ := client.VerifyWasCalledOnce().CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString(), AnyString()).GetCapturedArguments()
		expected := "Locks and plans deleted for the projects and workspaces modified in this pull request:\n\n" + "- dir: `.` workspace: `default`"
		Equals(t, expected, comment)

		logger.VerifyWasCalledOnce().Err("retrieving matching projects: %s", expetedErr)
	})

}
