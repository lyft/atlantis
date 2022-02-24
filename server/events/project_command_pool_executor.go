package events

import (
	"sync"

	"github.com/remeh/sizedwaitgroup"
	"github.com/runatlantis/atlantis/server/events/models"
)

type prjCmdRunnerFunc func(ctx models.ProjectCommandContext) models.ProjectResult

func runProjectCmdsParallel(
	cmds []models.ProjectCommandContext,
	runnerFunc prjCmdRunnerFunc,
	poolSize int,
) command.Result {
	var results []models.ProjectResult
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
	cmds []models.ProjectCommandContext,
	runnerFunc prjCmdRunnerFunc,
) command.Result {
	var results []models.ProjectResult
	for _, pCmd := range cmds {
		res := runnerFunc(pCmd)

		results = append(results, res)
	}
	return command.Result{ProjectResults: results}
}
