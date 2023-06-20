package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/palantir/go-githubapp/githubapp"
	runtime_models "github.com/runatlantis/atlantis/server/legacy/core/runtime/models"
	"github.com/runatlantis/atlantis/server/legacy/events"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/metrics"
	"github.com/runatlantis/atlantis/server/neptune/lyft/feature"
	"github.com/runatlantis/atlantis/server/vcs/provider/github"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/models"
)

type policyFilter interface {
	Filter(ctx context.Context, installationToken int64, repo models.Repo, prNum int, trigger command.CommandTrigger, failedPolicies []valid.PolicySet) ([]valid.PolicySet, error)
}

type exec interface {
	CombinedOutput(args []string, envs map[string]string, workdir string) (string, error)
}

const (
	conftestScope = "conftest.policies"
	// use internal server error message for user to understand error is from atlantis
	internalError = "internal server error"
)

// ConfTestExecutor runs a versioned conftest binary with the args built from the project context.
// Project context defines whether conftest runs a local policy set or runs a test on a remote policy set.
type ConfTestExecutor struct {
	Exec         exec
	PolicyFilter policyFilter
}

func NewConfTestExecutor(creator githubapp.ClientCreator, policySets valid.PolicySets, allocator feature.Allocator, logger logging.Logger) *ConfTestExecutor {
	reviewFetcher := &github.PRReviewFetcher{
		ClientCreator: creator,
	}
	reviewDismisser := &github.PRReviewDismisser{
		ClientCreator: creator,
	}
	teamMemberFetcher := &github.TeamMemberFetcher{
		ClientCreator: creator,
		Org:           policySets.Organization,
	}
	return &ConfTestExecutor{
		Exec:         runtime_models.LocalExec{},
		PolicyFilter: events.NewApprovedPolicyFilter(reviewFetcher, reviewDismisser, teamMemberFetcher, allocator, policySets.PolicySets, logger),
	}
}

// Run performs conftest policy tests against changes and fails if any policy does not pass. It also runs an all-or-nothing
// filter that will filter out all policy failures based on the filter criteria.
func (c *ConfTestExecutor) Run(_ context.Context, prjCtx command.ProjectContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) (string, error) {
	var policyNames []string
	var failedPolicies []valid.PolicySet
	var totalCmdOutput []string
	var policyErr error

	inputFile := filepath.Join(workdir, prjCtx.GetShowResultFileName())
	scope := prjCtx.Scope.SubScope(conftestScope)

	for _, policySet := range prjCtx.PolicySets.PolicySets {
		var policyArgs []Arg
		for _, path := range policySet.Paths {
			policyArgs = append(policyArgs, NewPolicyArg(path))
		}
		policyNames = append(policyNames, policySet.Name)
		args := ConftestTestCommandArgs{
			PolicyArgs: policyArgs,
			ExtraArgs:  extraArgs,
			InputFile:  inputFile,
			Command:    executablePath,
		}
		serializedArgs := args.build()
		policyScope := scope.SubScope(policySet.Name)
		cmdOutput, cmdErr := c.Exec.CombinedOutput(serializedArgs, envs, workdir)
		// Continue running other policies if one fails since it might not be the only failing one
		if cmdErr != nil {
			policyErr = cmdErr
			failedPolicies = append(failedPolicies, policySet)
			policyScope.Counter(metrics.ExecutionFailureMetric).Inc(1)
		} else {
			policyScope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
		}
		totalCmdOutput = append(totalCmdOutput, c.processOutput(cmdOutput, policySet, cmdErr))
	}

	title := c.buildTitle(policyNames)
	output := c.sanitizeOutput(inputFile, title+strings.Join(totalCmdOutput, "\n"))
	if prjCtx.InstallationToken == 0 {
		prjCtx.Log.ErrorContext(prjCtx.RequestCtx, "missing installation token")
		scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return output, errors.New(internalError)
	}

	failedPolicies, err := c.PolicyFilter.Filter(prjCtx.RequestCtx, prjCtx.InstallationToken, prjCtx.HeadRepo, prjCtx.Pull.Num, prjCtx.Trigger, failedPolicies)
	if err != nil {
		prjCtx.Log.ErrorContext(prjCtx.RequestCtx, fmt.Sprintf("error filtering out approved policies: %s", err.Error()))
		scope.Counter(metrics.ExecutionErrorMetric).Inc(1)
		return output, errors.New(internalError)
	}
	if len(failedPolicies) == 0 {
		scope.Counter(metrics.ExecutionSuccessMetric).Inc(1)
		return output, nil
	}
	// use policyErr here as policy error output is what the user should see
	scope.Counter(metrics.ExecutionFailureMetric).Inc(1)
	return output, policyErr
}

func (c *ConfTestExecutor) buildTitle(policySetNames []string) string {
	return fmt.Sprintf("Checking plan against the following policies: \n  %s\n\n", strings.Join(policySetNames, "\n  "))
}

func (c *ConfTestExecutor) sanitizeOutput(inputFile string, output string) string {
	return strings.Replace(output, inputFile, "<redacted plan file>", -1)
}

func (c *ConfTestExecutor) processOutput(output string, policySet valid.PolicySet, err error) string {
	// errored results need an extra newline
	if err != nil {
		return policySet.Name + ":\n" + output
	}
	return policySet.Name + ":" + output
}
