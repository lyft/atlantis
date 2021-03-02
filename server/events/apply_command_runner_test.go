package events_test

import (
	"errors"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/locking"
	lockingmocks "github.com/runatlantis/atlantis/server/events/locking/mocks"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

var applyLockCheckerMock lockingmocks.MockApplyLockChecker
var ctx *events.CommandContext

func TestApplyCommandLocker_IsDisabled(t *testing.T) {
	ctx := &events.CommandContext{
		Log: logging.NewNoopLogger(),
	}

	cases := []struct {
		Description      string
		DisableApply     bool
		ApplyLockPresent bool
		ApplyLockError   error
		ExpIsDisabled    bool
	}{
		{
			Description:      "When global apply lock is present IsDisabled returns true",
			DisableApply:     false,
			ApplyLockPresent: true,
			ApplyLockError:   nil,
			ExpIsDisabled:    true,
		},
		{
			Description:      "When no global apply lock is present and DisableApply flag is false IsDisabled returns false",
			DisableApply:     false,
			ApplyLockPresent: false,
			ApplyLockError:   nil,
			ExpIsDisabled:    false,
		},
		{
			Description:      "When no global apply lock is present and DisableApply flag is true IsDisabled returns true",
			ApplyLockPresent: false,
			DisableApply:     true,
			ApplyLockError:   nil,
			ExpIsDisabled:    true,
		},
		{
			Description:      "If ApplyLockChecker returns an error IsDisabled return value of DisableApply flag",
			ApplyLockError:   errors.New("error"),
			ApplyLockPresent: false,
			DisableApply:     true,
			ExpIsDisabled:    true,
		},
	}

	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			applyLockChecker := lockingmocks.NewMockApplyLockChecker()
			When(applyLockChecker.CheckApplyLock()).ThenReturn(locking.ApplyCommandLockResponse{Present: c.ApplyLockPresent}, nil)

			applyCommandLocker := events.NewApplyCommandLocker(applyLockChecker, c.DisableApply)

			Equals(t, c.ExpIsDisabled, applyCommandLocker.IsDisabled(ctx))
		})

	}
}
