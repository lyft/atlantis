package events

import (
	"sync"

	"github.com/remeh/sizedwaitgroup"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/command/project"
)

type prjCmdRunnerFunc func(ctx project.Context) project.Result

func runProjectCmdsParallel(
	cmds []project.Context,
	runnerFunc prjCmdRunnerFunc,
	poolSize int,
) command.Result {
	var results []project.Result
	mux := &sync.Mutex{}

	wg := sizedwaitgroup.New(poolSize)
	for _, pCmd := range cmds {
		pCmd := pCmd
		var execute func()
		wg.Add()

		execute = func() {
			defer wg.Done()
			res := runnerFunc(pCmd)
			mux.Lock()
			results = append(results, res)
			mux.Unlock()
		}

		go execute()
	}

	wg.Wait()
	return command.Result{ProjectResults: results}
}

func runProjectCmds(
	cmds []project.Context,
	runnerFunc prjCmdRunnerFunc,
) command.Result {
	var results []project.Result
	for _, pCmd := range cmds {
		res := runnerFunc(pCmd)

		results = append(results, res)
	}
	return command.Result{ProjectResults: results}
}
