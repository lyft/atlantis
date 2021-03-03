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

package locking_test

import (
	"errors"
	"testing"
	"time"

	"strings"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/locking"
	"github.com/runatlantis/atlantis/server/events/locking/mocks"
	"github.com/runatlantis/atlantis/server/events/locking/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

var project = models.NewProject("owner/repo", "path")
var workspace = "workspace"
var pull = models.PullRequest{}
var user = models.User{}
var errExpected = errors.New("err")
var timeNow = time.Now().Local()
var pl = models.ProjectLock{Project: project, Pull: pull, User: user, Workspace: workspace, Time: timeNow}

func TestTryLock_Err(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.TryLock(matchers.AnyModelsProjectLock())).ThenReturn(false, models.ProjectLock{}, errExpected)
	t.Log("when the backend returns an error, TryLock should return that error")
	l := locking.NewClient(backend)
	_, err := l.TryLock(project, workspace, pull, user)
	Equals(t, err, err)
}

func TestTryLock_Success(t *testing.T) {
	RegisterMockTestingT(t)
	currLock := models.ProjectLock{}
	backend := mocks.NewMockBackend()
	When(backend.TryLock(matchers.AnyModelsProjectLock())).ThenReturn(true, currLock, nil)
	l := locking.NewClient(backend)
	r, err := l.TryLock(project, workspace, pull, user)
	Ok(t, err)
	Equals(t, locking.TryLockResponse{LockAcquired: true, CurrLock: currLock, LockKey: "owner/repo/path/workspace"}, r)
}

func TestUnlock_InvalidKey(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	l := locking.NewClient(backend)

	_, err := l.Unlock("invalidkey")
	Assert(t, err != nil, "expected err")
	Assert(t, strings.Contains(err.Error(), "invalid key format"), "expected err")
}

func TestUnlock_Err(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.Unlock(matchers.AnyModelsProject(), AnyString())).ThenReturn(nil, errExpected)
	l := locking.NewClient(backend)
	_, err := l.Unlock("owner/repo/path/workspace")
	Equals(t, err, err)
	backend.VerifyWasCalledOnce().Unlock(project, "workspace")
}

func TestUnlock(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.Unlock(matchers.AnyModelsProject(), AnyString())).ThenReturn(&pl, nil)
	l := locking.NewClient(backend)
	lock, err := l.Unlock("owner/repo/path/workspace")
	Ok(t, err)
	Equals(t, &pl, lock)
}

func TestList_Err(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.List()).ThenReturn(nil, errExpected)
	l := locking.NewClient(backend)
	_, err := l.List()
	Equals(t, errExpected, err)
}

func TestList(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.List()).ThenReturn([]models.ProjectLock{pl}, nil)
	l := locking.NewClient(backend)
	list, err := l.List()
	Ok(t, err)
	Equals(t, map[string]models.ProjectLock{
		"owner/repo/path/workspace": pl,
	}, list)
}

func TestUnlockByPull(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.UnlockByPull("owner/repo", 1)).ThenReturn(nil, errExpected)
	l := locking.NewClient(backend)
	_, err := l.UnlockByPull("owner/repo", 1)
	Equals(t, errExpected, err)
}

func TestGetLock_BadKey(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	l := locking.NewClient(backend)
	_, err := l.GetLock("invalidkey")
	Assert(t, err != nil, "err should not be nil")
	Assert(t, strings.Contains(err.Error(), "invalid key format"), "expected different err")
}

func TestGetLock_Err(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.GetLock(project, workspace)).ThenReturn(nil, errExpected)
	l := locking.NewClient(backend)
	_, err := l.GetLock("owner/repo/path/workspace")
	Equals(t, errExpected, err)
}

func TestGetLock(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()
	When(backend.GetLock(project, workspace)).ThenReturn(&pl, nil)
	l := locking.NewClient(backend)
	lock, err := l.GetLock("owner/repo/path/workspace")
	Ok(t, err)
	Equals(t, &pl, lock)
}

func TestApplyLocker_LockCommand(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()

	t.Run("errors", func(t *testing.T) {
		When(backend.LockCommand(matchers.AnyModelsCommandName(), matchers.AnyTimeTime())).ThenReturn(nil, errExpected)
		l := locking.NewClient(backend)
		lock, err := l.LockApply()
		Equals(t, errExpected, err)
		Assert(t, !lock.Present, "exp false")
	})

	t.Run("succeeds", func(t *testing.T) {
		timeNow := time.Now()
		When(backend.LockCommand(matchers.AnyModelsCommandName(), matchers.AnyTimeTime())).ThenReturn(&models.CommandLock{Time: timeNow, CommandName: models.ApplyCommand}, nil)
		l := locking.NewClient(backend)
		lock, _ := l.LockApply()
		Assert(t, lock.Present, "exp lock present")
	})
}

func TestApplyLocker_UnlockCommand(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()

	t.Run("UnlockCommand fails", func(t *testing.T) {
		When(backend.UnlockCommand(matchers.AnyModelsCommandName())).ThenReturn(errExpected)
		l := locking.NewClient(backend)
		err := l.UnlockApply()
		Equals(t, errExpected, err)
	})

	t.Run("UnlockCommand succeeds", func(t *testing.T) {
		When(backend.UnlockCommand(matchers.AnyModelsCommandName())).ThenReturn(nil)
		l := locking.NewClient(backend)
		err := l.UnlockApply()
		Equals(t, nil, err)
	})

}

func TestApplyLocker_CheckApplyLock(t *testing.T) {
	RegisterMockTestingT(t)
	backend := mocks.NewMockBackend()

	t.Run("fails", func(t *testing.T) {
		When(backend.CheckCommandLock(matchers.AnyModelsCommandName())).ThenReturn(nil, errExpected)
		l := locking.NewClient(backend)
		lock, err := l.CheckApplyLock()
		Equals(t, errExpected, err)
		Equals(t, lock.Present, false)
	})

	t.Run("UnlockCommand succeeds", func(t *testing.T) {
		timeNow := time.Now()
		When(backend.CheckCommandLock(matchers.AnyModelsCommandName())).ThenReturn(&models.CommandLock{Time: timeNow, CommandName: models.ApplyCommand}, nil)
		l := locking.NewClient(backend)
		lock, err := l.CheckApplyLock()
		Equals(t, nil, err)
		Assert(t, lock.Present, "exp lock present")
	})

}
