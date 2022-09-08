package terraform

import "strings"

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

type CommandArguments struct {
	Command     Operation
	CommandArgs []string
	ExtraArgs   []string
}

func (t CommandArguments) Build() []string {
	return append([]string{t.Command.String()}, t.deDuplicateExtraArgs()...)
}

func stringInSlice(stringSlice []string, target string) bool {
	for _, value := range stringSlice {
		if value == target {
			return true
		}
	}
	return false
}

func (t CommandArguments) deDuplicateExtraArgs() []string {
	// work if any of the core args have been overridden
	finalArgs := []string{}
	usedExtraArgs := []string{}
	for _, arg := range t.CommandArgs {
		override := ""
		prefix := arg
		argSplit := strings.Split(arg, "=")
		if len(argSplit) == 2 {
			prefix = argSplit[0]
		}
		for _, extraArgOrig := range t.ExtraArgs {
			extraArg := extraArgOrig
			if strings.HasPrefix(extraArg, prefix) {
				override = extraArgOrig
				break
			}
			if strings.HasPrefix(extraArg, "--") {
				extraArg = extraArgOrig[1:]
				if strings.HasPrefix(extraArg, prefix) {
					override = extraArgOrig
					break
				}
			}
			if strings.HasPrefix(prefix, "--") {
				prefixWithoutDash := prefix[1:]
				if strings.HasPrefix(extraArg, prefixWithoutDash) {
					override = extraArgOrig
					break
				}
			}

		}
		if override != "" {
			finalArgs = append(finalArgs, override)
			usedExtraArgs = append(usedExtraArgs, override)
		} else {
			finalArgs = append(finalArgs, arg)
		}
	}
	// add any extra args that are not overrides
	for _, extraArg := range t.ExtraArgs {
		if !stringInSlice(usedExtraArgs, extraArg) {
			finalArgs = append(finalArgs, extraArg)
		}
	}
	return finalArgs
}
