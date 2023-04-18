package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

type Operation string

const (
	// Terraform operations
	TerraformInit  Operation = "init"
	TerraformPlan  Operation = "plan"
	TerraformApply Operation = "apply"
	TerraformShow  Operation = "show"

	// Conftest operations
	ConftestTest Operation = "test"
)

// Argument is the key value pair passed into the command
type Argument struct {
	Key   string
	Value string
}

func (a Argument) build() string {
	return fmt.Sprintf("-%s=%s", a.Key, a.Value)
}

// Flag is an argument with only a value
type Flag struct {
	Value string
}

func (f Flag) build() string {
	return fmt.Sprintf("-%s", f.Value)
}

// takes in a list of key/value argument pairs and parses them.
// terraform arguments are expected to be in a certain form
// ie. "-input=false" where input and false are the key values respectively.
func NewArgumentList(args []string) ([]Argument, error) {
	arguments := []Argument{}
	for _, arg := range args {
		typedArgument, err := newArgument(arg)
		if err != nil {
			return []Argument{}, errors.Wrap(err, "building argument list")
		}
		arguments = append(arguments, typedArgument)
	}
	return arguments, nil
}

func newArgument(arg string) (Argument, error) {
	// Remove any forward dashes and spaces
	arg = strings.TrimLeft(arg, "- ")
	coll := strings.Split(arg, "=")

	if len(coll) != 2 {
		return Argument{}, fmt.Errorf("cannot parse argument: %s. argument can only have one =", arg)
	}

	return Argument{
		Key:   coll[0],
		Value: coll[1],
	}, nil
}

type SubCommand struct {
	op    Operation
	input string
	args  []Argument
	flags []Flag
}

func NewSubCommand(op Operation) *SubCommand {
	return &SubCommand{
		op: op,
	}
}

// WithArgs adds args without de-duplicating them for
// commands that allow for multiple identical arg keys to exist
func (c *SubCommand) WithArgs(args ...Argument) *SubCommand {
	c.args = args
	return c
}

// WithUniqueArgs dedups incoming args using a "last one wins" approach
// and sets them appropriately on the receiver
func (c *SubCommand) WithUniqueArgs(args ...Argument) *SubCommand {
	c.args = c.dedup(args)
	return c
}

// WithInput adds a single command input
func (c *SubCommand) WithInput(input string) *SubCommand {
	c.input = input
	return c
}

func (c *SubCommand) WithFlags(flags ...Flag) *SubCommand {
	c.flags = flags
	return c
}

func (c *SubCommand) Build() []string {
	var result []string

	// first append operation
	result = append(result, string(c.op))

	// append all args
	for _, a := range c.args {
		result = append(result, a.build())
	}

	// append all flags
	for _, f := range c.flags {
		result = append(result, f.build())
	}

	// append input if it exists
	if c.input != "" {
		result = append(result, c.input)
	}

	return result
}

func (c *SubCommand) dedup(args []Argument) []Argument {
	tmp := map[string]string{}
	var finalArgs []Argument

	for _, a := range args {
		tmp[a.Key] = a.Value
	}

	// let's sort our keys to ensure a deterministic order
	// for testing at least
	var keys []string
	for k := range tmp {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		finalArgs = append(finalArgs, Argument{
			Key:   k,
			Value: tmp[k],
		})
	}

	return finalArgs
}
