package activities

import (
	"bytes"
	"context"
	"fmt"
	"github.com/hashicorp/go-version"
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
	Policies               []PolicySet
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
	//todo: support warn status
)

type ValidationResult struct {
	Status    ValidationStatus
	PolicySet PolicySet
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

	ch := c.StreamHandler.RegisterJob(request.JobID)
	defer close(ch)

	title := c.buildTitle()
	c.writeOutput(ch, title)

	// run each policy separately to track which pass and fail
	var validationResults []ValidationResult
	for _, policy := range c.Policies {
		// add paths as arguments
		var policyArgs []command.Argument
		for _, path := range policy.Paths {
			policyArgs = append(policyArgs, command.Argument{
				Key:   "p",
				Value: path,
			})
		}
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
		processedOutput := c.processOutput(cmdOutput, policy, showFile, cmdErr)
		c.writeOutput(ch, processedOutput)
	}
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

func (c *conftestActivity) writeOutput(ch chan string, output string) {
	ch <- output
}

func (c *conftestActivity) buildTitle() string {
	var policySetNames []string
	for _, policy := range c.Policies {
		policySetNames = append(policySetNames, policy.Name)
	}
	return fmt.Sprintf("Checking plan against the following policies: \n  %s\n\n", strings.Join(policySetNames, "\n  "))
}

func (c *conftestActivity) processOutput(output string, policySet PolicySet, inputFile string, err error) string {
	// errored results need an extra newline
	sanitizedOutput := strings.Replace(output, inputFile, "<redacted plan file>", -1)
	if err != nil {
		return policySet.Name + ":\n" + sanitizedOutput + "\n"
	}
	return policySet.Name + ":" + sanitizedOutput + "\n"
}
