package server_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

func TestUserConfig_DisableApply(t *testing.T) {
	t.Run("DisableApply is static and false", func(t *testing.T) {
		u := server.UserConfig{
			DisableApply: false,
		}
		Equals(t, false, u.IsApplyDisabled())
	})

	t.Run("DisableApply is static and true", func(t *testing.T) {
		u := server.UserConfig{
			DisableApply: true,
		}
		Equals(t, true, u.IsApplyDisabled())
	})

	t.Run("DisableApply is overriden", func(t *testing.T) {
		u := server.UserConfig{
			DisableApply: false,
		}

		Equals(t, true, u.IsApplyDisabled())
	})
}

func TestUserConfig_ToLogLevel(t *testing.T) {
	cases := []struct {
		userLvl string
		expLvl  logging.LogLevel
	}{
		{
			"debug",
			logging.Debug,
		},
		{
			"info",
			logging.Info,
		},
		{
			"warn",
			logging.Warn,
		},
		{
			"error",
			logging.Error,
		},
		{
			"unknown",
			logging.Info,
		},
	}

	for _, c := range cases {
		t.Run(c.userLvl, func(t *testing.T) {
			u := server.UserConfig{
				LogLevel: c.userLvl,
			}
			Equals(t, c.expLvl, u.ToLogLevel())
		})
	}
}
