package activities

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/events/terraform/ansi"
	"github.com/runatlantis/atlantis/server/neptune/terraform"
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

type TerraformClient interface {
	RunCommand(ctx context.Context, jobID string, path string, subcommand *terraform.SubCommand, customEnvVars map[string]string, v *version.Version) <-chan terraform.Line
}

type outputReader interface {
	Read(ctx context.Context, jobID string, ch <-chan terraform.Line) error
	Close(ctx context.Context, jobID string)
}

type terraformActivities struct {
	TerraformClient  TerraformClient
	DefaultTFVersion *version.Version
	OutputReader     outputReader
}

func NewTerraformActivities(client TerraformClient, defaultTfVersion *version.Version, outputHandler outputReader) *terraformActivities {
	return &terraformActivities{
		TerraformClient:  client,
		DefaultTFVersion: defaultTfVersion,
		OutputReader:     outputHandler,
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
	cmd := terraform.NewSubCommand(terraform.Init).WithArgs(args...)

	ch := t.TerraformClient.RunCommand(ctx, request.JobID, request.Path, cmd, request.Envs, tfVersion)

	// Read output and stream to active connections
	if err := t.OutputReader.Read(ctx, request.JobID, ch); err != nil {
		return TerraformInitResponse{}, errors.Wrap(err, "reading init output")
	}
	defer t.OutputReader.Close(ctx, request.JobID)
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
}

func (t *terraformActivities) TerraformPlan(ctx context.Context, request TerraformPlanRequest) (TerraformPlanResponse, error) {
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformPlanResponse{}, err
	}
	planFile := buildPlanFilePath(request.Path)

	args := []terraform.Argument{
		DisableInputArg,
		RefreshArg,
		{
			Key:   outArgKey,
			Value: planFile,
		},
	}
	args = append(args, request.Args...)

	cmd := terraform.NewSubCommand(terraform.Plan).WithArgs(args...)
	ch := t.TerraformClient.RunCommand(ctx, request.JobID, request.Path, cmd, request.Envs, tfVersion)

	// Read output and stream to active connections
	if err := t.OutputReader.Read(ctx, request.JobID, ch); err != nil {
		return TerraformPlanResponse{}, errors.Wrap(err, "reading plan output")
	}
	defer t.OutputReader.Close(ctx, request.JobID)

	return TerraformPlanResponse{
		PlanFile: planFile,
	}, nil
}

// Terraform Apply

type TerraformApplyRequest struct {
	Args      []terraform.Argument
	Envs      map[string]string
	JobID     string
	TfVersion string
	Path      string
}

type TerraformApplyResponse struct {
	Output string
}

func (t *terraformActivities) TerraformApply(ctx context.Context, request TerraformApplyRequest) (TerraformApplyResponse, error) {
	// Fail requests using a target flag, as Atlantis cannot support this functionality
	if containsTargetFlag(request.Args) {
		return TerraformApplyResponse{}, errors.New("request contains invalid -target flag")
	}

	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformApplyResponse{}, err
	}

	planFile := buildPlanFilePath(request.Path)
	args := []terraform.Argument{DisableInputArg}
	args = append(args, request.Args...)

	cmd := terraform.NewSubCommand(terraform.Apply).WithInput(planFile).WithArgs(args...)
	ch := t.TerraformClient.RunCommand(ctx, request.JobID, request.Path, cmd, request.Envs, tfVersion)

	// Read output and stream to active connections
	if err := t.OutputReader.Read(ctx, request.JobID, ch); err != nil {
		return TerraformApplyResponse{}, errors.Wrap(err, "reading apply output")
	}
	defer t.OutputReader.Close(ctx, request.JobID)

	return TerraformApplyResponse{}, nil
}

func containsTargetFlag(args []terraform.Argument) bool {
	for _, arg := range args {
		if arg.Key == "target" {
			return true
		}
	}
	return false
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

func (t *terraformActivities) readCommandOutput(ch <-chan terraform.Line) (string, error) {
	var err error
	var lines []string
	for line := range ch {
		if line.Err != nil {
			err = errors.Wrap(line.Err, "executing command")
			break
		}
		lines = append(lines, line.Line)
	}
	if err != nil {
		return "", err
	}
	output := strings.Join(lines, "\n")
	// sanitize output by stripping out any ansi characters.
	output = ansi.Strip(output)
	return output, nil
}

func buildPlanFilePath(path string) string {
	return filepath.Join(path, PlanOutputFile)
}
