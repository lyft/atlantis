package policy

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"path/filepath"
	"strings"

	runtime_models "github.com/runatlantis/atlantis/server/core/runtime/models"
	"github.com/runatlantis/atlantis/server/events/command"
)

type policyFilter interface {
	Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error)
}

// ConfTestExecutor runs a versioned conftest binary with the args built from the project context.
// Project context defines whether conftest runs a local policy set or runs a test on a remote policy set.
type ConfTestExecutor struct {
	SourceResolver SourceResolver
	Exec           runtime_models.Exec
	policyFilter   policyFilter
}

func (c *ConfTestExecutor) Run(ctx context.Context, prjCtx command.ProjectContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) (string, error) {
	var policyNames []string
	var failedPolicies []valid.PolicySet
	var results []string
	var policyErr error

	inputFile := filepath.Join(workdir, prjCtx.GetShowResultFileName())

	for _, policySet := range prjCtx.PolicySets.PolicySets {
		path, err := c.SourceResolver.Resolve(policySet)
		// Let's not fail the whole step because of a single failure. Log and fail silently
		if err != nil {
			prjCtx.Log.ErrorContext(prjCtx.RequestCtx, "Error resolving policyset", map[string]interface{}{
				"policy": policySet.Name,
				"err":    err.Error(),
			})
			continue
		}
		args := ConftestTestCommandArgs{
			PolicyArgs: []Arg{NewPolicyArg(path)},
			ExtraArgs:  extraArgs,
			InputFile:  inputFile,
			Command:    executablePath,
		}
		serializedArgs, err := args.build()
		if err != nil {
			prjCtx.Log.WarnContext(prjCtx.RequestCtx, "No policies have been configured")
			return "", nil
			// TODO: enable when we can pass policies in otherwise e2e tests with policy checks fail
			// return "", errors.Wrap(err, "building args")
		}
		policyNames = append(policyNames, policySet.Name)
		cmdOutput, err := c.Exec.CombinedOutput(serializedArgs, envs, workdir)
		if err != nil {
			failedPolicies = append(failedPolicies, policySet)
			policyErr = err
		}
		results = append(results, cmdOutput)
	}
	title := c.buildTitle(policyNames)
	results = append([]string{title}, results...)
	output := c.sanitizeOutput(inputFile, strings.Join(results, "\n"))
	// TODO: populate installation token into project context
	failedPolicyNames, err := c.policyFilter.Filter(ctx, 0, prjCtx.HeadRepo, prjCtx.Pull.Num, failedPolicies)
	if err != nil {
		prjCtx.Log.ErrorContext(prjCtx.RequestCtx, "error filtering out approved policies", map[string]interface{}{
			"err": err,
		})
		return output, errors.New("internal server error")
	}
	if len(failedPolicyNames) == 0 {
		return output, nil
	}
	return output, policyErr
}

func (c *ConfTestExecutor) buildTitle(policySetNames []string) string {
	return fmt.Sprintf("Checking plan against the following policies: \n  %s\n", strings.Join(policySetNames, "\n  "))
}

func (c *ConfTestExecutor) sanitizeOutput(inputFile string, output string) string {
	return strings.Replace(output, inputFile, "<redacted plan file>", -1)
}
