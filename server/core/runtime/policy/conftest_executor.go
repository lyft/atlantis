package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

type sourceResolver interface {
	Resolve(policySet valid.PolicySet) (string, error)
}

type policyFilter interface {
	Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error)
}

type exec interface {
	CombinedOutput(args []string, envs map[string]string, workdir string) (string, error)
}

// ConfTestExecutor runs a versioned conftest binary with the args built from the project context.
// Project context defines whether conftest runs a local policy set or runs a test on a remote policy set.
type ConfTestExecutor struct {
	SourceResolver sourceResolver
	Exec           exec
	PolicyFilter   policyFilter
}

// Run performs conftest policy tests against changes and fails if any policy does not pass. It also runs an all-or-nothing
// filter that will filter out all policy failures based on the filter criteria.
func (c *ConfTestExecutor) Run(ctx context.Context, prjCtx command.ProjectContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) (string, error) {
	var policyArgs []Arg
	var policyNames []string
	var failedPolicies []valid.PolicySet
	inputFile := filepath.Join(workdir, prjCtx.GetShowResultFileName())

	for _, policySet := range prjCtx.PolicySets.PolicySets {
		path, err := c.SourceResolver.Resolve(policySet)
		// Let's not fail the whole step because of a single failure. Log and fail silently
		if err != nil {
			prjCtx.Log.ErrorContext(prjCtx.RequestCtx, fmt.Sprintf("Error resolving policyset %s. err: %s", policySet.Name, err.Error()))
			continue
		}
		policyArgs = append(policyArgs, NewPolicyArg(path))
		policyNames = append(policyNames, policySet.Name)
	}

	args := ConftestTestCommandArgs{
		PolicyArgs: policyArgs,
		ExtraArgs:  extraArgs,
		InputFile:  inputFile,
		Command:    executablePath,
	}
	serializedArgs, err := args.build()
	if err != nil {
		prjCtx.Log.WarnContext(prjCtx.RequestCtx, "No policies have been configured")
		return "", errors.Wrap(err, "building args")
	}

	// TODO: run each policy set separately and use each pass/failure decision to populate failedPolicies
	cmdOutput, policyErr := c.Exec.CombinedOutput(serializedArgs, envs, workdir)
	if policyErr != nil {
		failedPolicies = prjCtx.PolicySets.PolicySets
	}

	title := c.buildTitle(policyNames)
	output := c.sanitizeOutput(inputFile, title+cmdOutput)
	// TODO: populate installation token into project context
	failedPolicies, err = c.PolicyFilter.Filter(ctx, 0, prjCtx.HeadRepo, prjCtx.Pull.Num, failedPolicies)
	if err != nil {
		prjCtx.Log.ErrorContext(prjCtx.RequestCtx, "error filtering out approved policies", map[string]interface{}{
			"err": err,
		})
		return output, errors.New("internal server error")
	}
	if len(failedPolicies) == 0 {
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
