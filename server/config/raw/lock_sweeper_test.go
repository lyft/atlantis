package raw_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/config/raw"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enabledSweeper() raw.LockSweeper {
	return raw.LockSweeper{
		Enabled:     true,
		Bucket:      "atlantis-production-states",
		KeyPrefix:   "lyft/dsterraform",
		Repository:  "lyft/dsterraform",
		Table:       "atlantis-production-states-locks",
		OwnerPrefix: "atlantis-svc@worker",
	}
}

func TestLockSweeper_Validate(t *testing.T) {
	t.Run("disabled needs nothing", func(t *testing.T) {
		require.NoError(t, raw.LockSweeper{}.Validate())
	})

	t.Run("enabled with all fields is valid", func(t *testing.T) {
		require.NoError(t, enabledSweeper().Validate())
	})

	t.Run("enabled requires every target field", func(t *testing.T) {
		for name, mutate := range map[string]func(*raw.LockSweeper){
			"bucket":       func(l *raw.LockSweeper) { l.Bucket = "" },
			"key_prefix":   func(l *raw.LockSweeper) { l.KeyPrefix = "" },
			"repository":   func(l *raw.LockSweeper) { l.Repository = "" },
			"table":        func(l *raw.LockSweeper) { l.Table = "" },
			"owner_prefix": func(l *raw.LockSweeper) { l.OwnerPrefix = "" },
		} {
			l := enabledSweeper()
			mutate(&l)
			assert.Error(t, l.Validate(), name)
		}
	})

	t.Run("min age below the longest activity is rejected", func(t *testing.T) {
		l := enabledSweeper()
		l.MinAgeMinutes = 30
		assert.Error(t, l.Validate())
	})

	t.Run("min age at the floor is accepted", func(t *testing.T) {
		l := enabledSweeper()
		l.MinAgeMinutes = 60
		assert.NoError(t, l.Validate())
	})
}

func TestLockSweeper_ToValid(t *testing.T) {
	t.Run("disabled converts to the zero value", func(t *testing.T) {
		assert.Equal(t, valid.LockSweeper{}, raw.LockSweeper{Bucket: "ignored"}.ToValid())
	})

	t.Run("defaults to dry run with a 2 hour threshold", func(t *testing.T) {
		v := enabledSweeper().ToValid()
		assert.True(t, v.DryRun)
		assert.Equal(t, 120, v.MinAgeMinutes)
		assert.Equal(t, "atlantis-svc@worker", v.OwnerPrefix)
		assert.True(t, v.Enabled)
	})

	t.Run("only an explicit false turns dry run off", func(t *testing.T) {
		off := false
		l := enabledSweeper()
		l.DryRun = &off
		assert.False(t, l.ToValid().DryRun)
	})

	t.Run("an explicit threshold is kept", func(t *testing.T) {
		l := enabledSweeper()
		l.MinAgeMinutes = 240
		assert.Equal(t, 240, l.ToValid().MinAgeMinutes)
	})
}
