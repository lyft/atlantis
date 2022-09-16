package activities

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/terraform/ansi"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/logger"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities/terraform"
)

var DisableInputArg = terraform.Argument{
	Key:   "input",
	Value: "false",
}

var RefreshArg = terraform.Argument{
	Key:   "refresh",
	Value: "true",
}

const (
	outArgKey      = "out"
	PlanOutputFile = "output.tfplan"
)

// Setting the buffer size to 10mb
const bufioScannerBufferSize = 10 * 1024 * 1024

type TerraformClient interface {
	RunCommand(ctx context.Context, request *terraform.RunCommandRequest, options ...terraform.RunOptions) error
}

type terraformActivities struct {
	TerraformClient  TerraformClient
	DefaultTFVersion *version.Version
}

func NewTerraformActivities(client TerraformClient, defaultTfVersion *version.Version) *terraformActivities {
	return &terraformActivities{
		TerraformClient:  client,
		DefaultTFVersion: defaultTfVersion,
	}
}

// Terraform Init
type TerraformInitRequest struct {
	Args      []terraform.Argument
	Envs      map[string]string
	JobID     string
	TfVersion string
	Path      string
}

type TerraformInitResponse struct {
	Output string
}

func (t *terraformActivities) TerraformInit(ctx context.Context, request TerraformInitRequest) (TerraformInitResponse, error) {
	// Resolve the tf version to be used for this operation
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformInitResponse{}, err
	}

	args := []terraform.Argument{
		DisableInputArg,
	}
	args = append(args, request.Args...)

	r := &terraform.RunCommandRequest{
		RootPath:          request.Path,
		SubCommand:        terraform.NewSubCommand(terraform.Init).WithArgs(args...),
		AdditionalEnvVars: request.Envs,
		Version:           tfVersion,
	}
	err = t.runCommandWithOutputStream(ctx, r)
	if err != nil {
		return TerraformInitResponse{}, errors.Wrap(err, "running init command")
	}
	return TerraformInitResponse{}, nil
}

// Terraform Plan
type TerraformPlanRequest struct {
	Args      []terraform.Argument
	Envs      map[string]string
	JobID     string
	TfVersion string
	Path      string
}

type TerraformPlanResponse struct {
	PlanFile string
	Output   string
	Summary  terraform.PlanSummary
}

func (t *terraformActivities) TerraformPlan(ctx context.Context, request TerraformPlanRequest) (TerraformPlanResponse, error) {
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformPlanResponse{}, err
	}
	planFile := filepath.Join(request.Path, PlanOutputFile)
	args := []terraform.Argument{
		DisableInputArg,
		RefreshArg,
		{
			Key:   outArgKey,
			Value: planFile,
		},
	}
	args = append(args, request.Args...)

	planRequest := &terraform.RunCommandRequest{
		RootPath:          request.Path,
		SubCommand:        terraform.NewSubCommand(terraform.Plan).WithArgs(args...),
		AdditionalEnvVars: request.Envs,
		Version:           tfVersion,
	}
	err = t.runCommandWithOutputStream(ctx, planRequest)
	if err != nil {
		return TerraformPlanResponse{}, errors.Wrap(err, "running plan command")
	}

	// let's run terraform show right after to get the plan as a structured object
	showRequest := &terraform.RunCommandRequest{
		RootPath: request.Path,
		SubCommand: terraform.NewSubCommand(terraform.Show).WithFlags(terraform.Flag{
			Value: "json",
		}),
		AdditionalEnvVars: request.Envs,
		Version:           tfVersion,
	}

	showResultBuffer := &bytes.Buffer{}
	err = t.TerraformClient.RunCommand(ctx, showRequest, terraform.RunOptions{
		StdOut: showResultBuffer,
		StdErr: showResultBuffer,
	})

	// we shouldn't fail our activity just because show failed. Summaries aren't that critical.
	if err != nil {
		logger.Error(ctx, "error with terraform show", "err", err)
	}

	summary, err := terraform.NewPlanSummaryFromJSON(showResultBuffer.Bytes())

	if err != nil {
		logger.Error(ctx, "error building plan summary", "err", err)
	}

	return TerraformPlanResponse{
		PlanFile: filepath.Join(request.Path, PlanOutputFile),
		Summary:  summary,
	}, nil
}

func (t *terraformActivities) runCommandWithOutputStream(ctx context.Context, request *terraform.RunCommandRequest) error {
	reader, writer := io.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	var err error
	go func() {
		defer wg.Done()
		defer writer.Close()
		err = t.TerraformClient.RunCommand(ctx, request, terraform.RunOptions{
			StdOut: writer,
			StdErr: writer,
		})
	}()

	s := bufio.NewScanner(reader)

	buf := []byte{}
	s.Buffer(buf, bufioScannerBufferSize)

	for s.Scan() {
		// TODO: forward to global channel
		// message := s.Text()
		// ch <- terraform.Line{Line: message}
	}

	wg.Wait()

	return err
}

// Terraform Apply

type TerraformApplyRequest struct {
}

func (t *terraformActivities) TerraformApply(ctx context.Context, request TerraformApplyRequest) error {
	return nil
}

func (t *terraformActivities) resolveVersion(v string) (*version.Version, error) {
	// Use default version if configured version is empty
	if v == "" {
		return t.DefaultTFVersion, nil
	}

	version, err := version.NewVersion(v)
	if err != nil {
		return nil, errors.Wrap(err, "resolving terraform version")
	}

	if version != nil {
		return version, nil
	}
	return t.DefaultTFVersion, nil
}

func (t *terraformActivities) readCommandOutput(ch <-chan terraform.Line) string {
	var lines []string
	for line := range ch {
		lines = append(lines, line.Line)
	}
	output := strings.Join(lines, "\n")
	// sanitize output by stripping out any ansi characters.
	output = ansi.Strip(output)
	return output
}
