package command_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"
	"github.com/stretchr/testify/assert"
)

func TestCommandArguments_Build(t *testing.T) {
	t.Run("with flags", func(t *testing.T) {
		c := command.NewSubCommand(command.Show)

		c.WithFlags(command.Flag{
			Value: "json",
		})

		assert.Equal(t, []string{"show", "-json"}, c.Build())
	})

	t.Run("with input", func(t *testing.T) {
		c := command.NewSubCommand(command.Apply)

		c.WithInput("input.tfplan")

		assert.Equal(t, []string{"apply", "input.tfplan"}, c.Build())
	})

	t.Run("with args", func(t *testing.T) {
		c := command.NewSubCommand(command.Init)

		c.WithArgs(command.Argument{
			Key:   "input",
			Value: "false",
		})

		assert.Equal(t, []string{"init", "-input=false"}, c.Build())
	})

	t.Run("dedups last first", func(t *testing.T) {
		c := command.NewSubCommand(command.Init)

		c.WithArgs(
			command.Argument{
				Key:   "input",
				Value: "false",
			},
			command.Argument{
				Key:   "a",
				Value: "b",
			},
			command.Argument{
				Key:   "input",
				Value: "true",
			},
			command.Argument{
				Key:   "c",
				Value: "d",
			},
		)

		assert.Equal(t, []string{"init", "-a=b", "-c=d", "-input=true"}, c.Build())
	})
}
