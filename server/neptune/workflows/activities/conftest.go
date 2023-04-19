package activities

import (
	"bytes"
	"context"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/temporal"
	"go.temporal.io/sdk/activity"
	"strings"
)

type asyncClient interface {
	RunCommand(ctx context.Context, request *command.RunCommandRequest, options ...command.RunOptions) error
}

type fileValidator interface {
	Stat(name string) error
}

var NoColorFlag = command.Flag{
	Value: "-no-color",
}

type conftestActivity struct {
	DefaultConftestVersion *version.Version
	ConftestClient         asyncClient
	StreamHandler          streamer
	Policies               valid.PolicySets
	FileValidator          fileValidator
}

type ConftestRequest struct {
	Args        []command.Argument
	DynamicEnvs []EnvVar
	JobID       string
	Path        string
	ShowFile    string
}

type ValidationStatus int

const (
	Success ValidationStatus = iota
	Fail
)

type ValidationResult struct {
	Status    ValidationStatus
	PolicySet valid.PolicySet
}

type ConftestResponse struct {
	ValidationResults []ValidationResult
}

func (c *conftestActivity) Conftest(ctx context.Context, request ConftestRequest) (ConftestResponse, error) {
	cancel := temporal.StartHeartbeat(ctx, temporal.HeartbeatTimeout)
	defer cancel()

	// validate terraform show file exists
	showFile := request.ShowFile
	if err := c.FileValidator.Stat(showFile); err != nil {
		return ConftestResponse{}, err
	}

	envs, err := getEnvs(request.DynamicEnvs)
	if err != nil {
		return ConftestResponse{}, err
	}

	var policyNames []string
	var totalCmdOutput []string
	var validationResults []ValidationResult

	// run each policy separately to track which pass and fail
	for _, policy := range c.Policies.PolicySets {
		// add paths as arguments
		var policyArgs []command.Argument
		for _, path := range policy.Paths {
			policyArgs = append(policyArgs, command.Argument{
				Key:   "p",
				Value: path,
			})
		}
		policyNames = append(policyNames, policy.Name)
		args := append(policyArgs, request.Args...)
		conftestRequest := &command.RunCommandRequest{
			RootPath:          request.Path,
			SubCommand:        command.NewSubCommand(command.ConftestTest).WithInput(showFile).WithFlags(NoColorFlag).WithArgs(args...),
			AdditionalEnvVars: envs,
			Version:           c.DefaultConftestVersion,
		}
		cmdOutput, cmdErr := c.runCommand(ctx, conftestRequest)
		// Continue running other policies if one fails since it might not be the only failing one
		if cmdErr != nil {
			activity.GetLogger(ctx).Error(cmdOutput)
			validationResults = append(validationResults, ValidationResult{
				Status:    Fail,
				PolicySet: policy,
			})
		} else {
			validationResults = append(validationResults, ValidationResult{
				Status:    Success,
				PolicySet: policy,
			})
		}
		totalCmdOutput = append(totalCmdOutput, c.processOutput(cmdOutput, policy, cmdErr))
	}
	title := c.buildTitle(policyNames)
	output := c.sanitizeOutput(showFile, title+strings.Join(totalCmdOutput, "\n"))
	c.writeOutput(output, request.JobID)
	return ConftestResponse{ValidationResults: validationResults}, nil
}

func (c *conftestActivity) runCommand(ctx context.Context, request *command.RunCommandRequest) (string, error) {
	buf := &bytes.Buffer{}
	err := c.ConftestClient.RunCommand(ctx, request, command.RunOptions{
		StdOut: buf,
		StdErr: buf,
	})
	return buf.String(), err
}

// TODO: instead of just writing everything at the end, actually stream results as we run each policy
// this is a simple MVP test to first see how it looks in the UI
func (c *conftestActivity) writeOutput(output string, jobID string) {
	ch := c.StreamHandler.RegisterJob(jobID)
	ch <- output
	close(ch)
}

func (c *conftestActivity) buildTitle(policySetNames []string) string {
	return fmt.Sprintf("Checking plan against the following policies: \n  %s\n\n", strings.Join(policySetNames, "\n  "))
}

func (c *conftestActivity) sanitizeOutput(inputFile string, output string) string {
	return strings.Replace(output, inputFile, "<redacted plan file>", -1)
}

func (c *conftestActivity) processOutput(output string, policySet valid.PolicySet, err error) string {
	// errored results need an extra newline
	if err != nil {
		return policySet.Name + ":\n" + output
	}
	return policySet.Name + ":" + output
}
