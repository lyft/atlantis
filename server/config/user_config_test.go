package config_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

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
			u := config.UserConfig{
				LogLevel: c.userLvl,
			}
			Equals(t, c.expLvl, logging.ToLogLevel(u.LogLevel))
		})
	}
}
