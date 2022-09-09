package terraform

import (
	"errors"
	"fmt"
	"strings"
)

type Operation int

const (
	Init Operation = iota
	Plan
	Apply
)

func (t Operation) String() string {
	switch t {
	case Init:
		return "init"
	case Plan:
		return "plan"
	case Apply:
		return "apply"
	}
	return ""
}

// Argument is the key value pair passed into the terraform command
type Argument struct {
	Key   string
	Value string
}

func (a Argument) build() string {
	return fmt.Sprintf("-%s=%s", a.Key, a.Value)
}

// for building extra args from string
func NewArgumentListFrom(args []string) ([]Argument, error) {
	arguments := []Argument{}
	for _, arg := range args {
		argument, err := NewArgumentFrom(arg)
		if err != nil {
			return []Argument{}, err
		}
		arguments = append(arguments, argument)
	}
	return arguments, nil
}

func NewArgumentFrom(arg string) (Argument, error) {
	// Remove any forward dashes
	arg = strings.TrimLeft(arg, "- ")
	coll := strings.Split(arg, "=")

	if len(coll) != 2 {
		return Argument{}, errors.New("cannot parse argument")
	}

	return Argument{
		Key:   coll[0],
		Value: coll[1],
	}, nil
}

type CommandArguments struct {
	Command     Operation
	CommandArgs []Argument
	ExtraArgs   []Argument
}

func isArgKeyInArgsList(arg Argument, args []Argument) bool {
	for _, a := range args {
		if a.Key == arg.Key {
			return true
		}
	}
	return false
}

func (t CommandArguments) Build() []string {
	finalArgs := []string{}
	for _, arg := range t.CommandArgs {

		overrideIndex := -1
		for i, overrideArg := range t.ExtraArgs {
			if overrideArg.Key == arg.Key {
				overrideIndex = i
			}
		}

		// Override argument exists
		if overrideIndex != -1 {
			finalArgs = append(finalArgs, t.ExtraArgs[overrideIndex].build())
		}
	}

	return append([]string{t.Command.String()}, finalArgs...)
}
