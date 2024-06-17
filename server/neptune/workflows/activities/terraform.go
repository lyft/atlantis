package activities

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"

	key "github.com/runatlantis/atlantis/server/neptune/context"
	"go.temporal.io/sdk/activity"

	"github.com/hashicorp/go-version"
	"github.com/palantir/go-githubapp/githubapp"
	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/file"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/temporal"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

const (
	TFInAutomation                        = "TF_IN_AUTOMATION"
	TFInAutomationVal                     = "true"
	AtlantisTerraformVersion              = "ATLANTIS_TERRAFORM_VERSION"
	Dir                                   = "DIR"
	TFPluginCacheDir                      = "TF_PLUGIN_CACHE_DIR"
	PluginCacheMayBreakDependencyLockFile = "TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE"
)

// TerraformClientError can be used to assert a non-retryable error type for
// callers of this activity
type TerraformClientError struct {
	err error
}

func (e TerraformClientError) Error() string {
	return e.err.Error()
}

func NewTerraformClientError(err error) *TerraformClientError {
	return &TerraformClientError{
		err: err,
	}
}

func wrapTerraformError(err error, message string) TerraformClientError {
	// double wrap here to get specifics + error type for temporal to not retry
	return TerraformClientError{
		err: errors.Wrap(err, message),
	}
}

var DisableInputArg = command.Argument{
	Key:   "input",
	Value: "false",
}

var RefreshArg = command.Argument{
	Key:   "refresh",
	Value: "true",
}

const (
	outArgKey          = "out"
	PlanOutputFile     = "output.tfplan"
	PlanOutputJSONFile = "output.json"
)

// Setting the buffer size to 10mb
const bufioScannerBufferSize = 10 * 1024 * 1024

type TerraformClient interface {
	RunCommand(ctx context.Context, request *command.RunCommandRequest, options ...command.RunOptions) error
}

type streamer interface {
	RegisterJob(id string) chan string
}

type gitCredentialsRefresher interface {
	Refresh(ctx context.Context, token int64) error
}

type writer interface {
	Write(name string, data []byte) error
}

type terraformActivities struct {
	TerraformClient        TerraformClient
	DefaultTFVersion       *version.Version
	StreamHandler          streamer
	GHAppConfig            githubapp.Config
	GitCLICredentials      gitCredentialsRefresher
	GitCredentialsFileLock *file.RWLock
	FileWriter             writer
	CacheDir               string
	InstallationID         int64
}

func NewTerraformActivities(
	client TerraformClient,
	defaultTfVersion *version.Version,
	streamHandler streamer,
	gitCredentialsRefresher gitCredentialsRefresher,
	gitCredentialsFileLock *file.RWLock,
	fileWriter writer,
	cacheDir string,
	installationID int64,
) *terraformActivities { //nolint:revive // avoiding refactor while adding linter action
	return &terraformActivities{
		TerraformClient:        client,
		DefaultTFVersion:       defaultTfVersion,
		StreamHandler:          streamHandler,
		GitCLICredentials:      gitCredentialsRefresher,
		GitCredentialsFileLock: gitCredentialsFileLock,
		InstallationID:         installationID,
		FileWriter:             fileWriter,
		CacheDir:               cacheDir,
	}
}

func getEnvs(dynamicEnvs []EnvVar) (map[string]string, error) {
	envs := make(map[string]string)
	for _, e := range dynamicEnvs {
		v, err := e.GetValue()

		if err != nil {
			return envs, errors.Wrap(err, fmt.Sprintf("loading dynamic env var with name %s", e.Name))
		}

		envs[e.Name] = v
	}

	return envs, nil
}

// Terraform Init
type TerraformInitRequest struct {
	Args                 []command.Argument
	DynamicEnvs          []EnvVar
	JobID                string
	TfVersion            string
	Path                 string
	GithubInstallationID int64
}

type TerraformInitResponse struct {
	Output string
}

func (t *terraformActivities) TerraformInit(ctx context.Context, request TerraformInitRequest) (TerraformInitResponse, error) {
	cancel := temporal.StartHeartbeat(ctx, temporal.HeartbeatTimeout)
	defer cancel()

	// Resolve the tf version to be used for this operation
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformInitResponse{}, err
	}

	args := []command.Argument{
		DisableInputArg,
	}
	args = append(args, request.Args...)

	envs, err := getEnvs(request.DynamicEnvs)
	if err != nil {
		return TerraformInitResponse{}, err
	}
	t.addTerraformEnvs(envs, request.Path, tfVersion)

	r := &command.RunCommandRequest{
		RootPath:          request.Path,
		SubCommand:        command.NewSubCommand(command.TerraformInit).WithUniqueArgs(args...),
		AdditionalEnvVars: envs,
		Version:           tfVersion,
	}

	err = t.GitCLICredentials.Refresh(ctx, t.InstallationID)
	if err != nil {
		activity.GetLogger(ctx).Error("Error refreshing git cli credentials. This is bug and will likely cause fetching of private modules to fail", key.ErrKey, err)
	}

	// terraform init clones repos using git cli auth of which we chose git global configs.
	// let's ensure we are locking access to this file so it's not rewritten to during the duration of our
	// operation
	t.GitCredentialsFileLock.RLock()
	defer t.GitCredentialsFileLock.RUnlock()

	out, err := t.runCommandWithOutputStream(ctx, request.JobID, r)
	if err != nil {
		activity.GetLogger(ctx).Error(out)
		return TerraformInitResponse{}, wrapTerraformError(err, "running init command")
	}
	return TerraformInitResponse{}, nil
}

// Terraform Plan

type TerraformPlanRequest struct {
	Args         []command.Argument
	DynamicEnvs  []EnvVar
	JobID        string
	TfVersion    string
	Path         string
	PlanMode     *terraform.PlanMode
	WorkflowMode terraform.WorkflowMode
}

type TerraformPlanResponse struct {
	PlanFile     string
	PlanJSONFile string
	Summary      terraform.PlanSummary
}

func (t *terraformActivities) TerraformPlan(ctx context.Context, request TerraformPlanRequest) (TerraformPlanResponse, error) {
	cancel := temporal.StartHeartbeat(ctx, temporal.HeartbeatTimeout)
	defer cancel()
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformPlanResponse{}, err
	}
	planFile := filepath.Join(request.Path, PlanOutputFile)

	args := []command.Argument{
		DisableInputArg,
		RefreshArg,
		{
			Key:   outArgKey,
			Value: planFile,
		},
	}
	args = append(args, request.Args...)
	var flags []command.Flag

	if request.PlanMode != nil {
		flags = append(flags, request.PlanMode.ToFlag())
	}

	envs, err := getEnvs(request.DynamicEnvs)
	if err != nil {
		return TerraformPlanResponse{}, err
	}
	t.addTerraformEnvs(envs, request.Path, tfVersion)

	planRequest := &command.RunCommandRequest{
		RootPath:          request.Path,
		SubCommand:        command.NewSubCommand(command.TerraformPlan).WithUniqueArgs(args...).WithFlags(flags...),
		AdditionalEnvVars: envs,
		Version:           tfVersion,
	}
	out, err := t.runCommandWithOutputStream(ctx, request.JobID, planRequest)

	if err != nil {
		activity.GetLogger(ctx).Error(out)
		return TerraformPlanResponse{}, wrapTerraformError(err, "running plan command")
	}

	// let's run terraform show right after to get the plan as a structured object
	showRequest := &command.RunCommandRequest{
		RootPath: request.Path,
		SubCommand: command.NewSubCommand(command.TerraformShow).
			WithFlags(command.Flag{
				Value: "json",
			}).
			WithInput(planFile),
		AdditionalEnvVars: envs,
		Version:           tfVersion,
	}

	showResultBuffer := &bytes.Buffer{}
	showErr := t.TerraformClient.RunCommand(ctx, showRequest, command.RunOptions{
		StdOut: showResultBuffer,
		StdErr: showResultBuffer,
	})

	// if used by the validate step, we will fail when we can't find the file
	if showErr != nil {
		activity.GetLogger(ctx).Error("error with terraform show", key.ErrKey, err)
	}

	showResults := showResultBuffer.Bytes()

	// write show results to disk
	var planJSONFile string
	if showErr == nil && request.WorkflowMode == terraform.PR {
		planJSONFile = filepath.Join(request.Path, PlanOutputJSONFile)
		if err = t.FileWriter.Write(planJSONFile, showResults); err != nil {
			activity.GetLogger(ctx).Error("error writing show results to disk", key.ErrKey, err)
		}
	}

	// build plan summaries
	summary, err := terraform.NewPlanSummaryFromJSON(showResults)
	if err != nil {
		activity.GetLogger(ctx).Error("error building plan summary", key.ErrKey, err)
	}

	return TerraformPlanResponse{
		PlanFile:     planFile,
		PlanJSONFile: planJSONFile,
		Summary:      summary,
	}, nil
}

// Terraform Apply

type TerraformApplyRequest struct {
	Args        []command.Argument
	DynamicEnvs []EnvVar
	JobID       string
	TfVersion   string
	Path        string
	PlanFile    string
}

type TerraformApplyResponse struct {
	Output string
}

func (t *terraformActivities) TerraformApply(ctx context.Context, request TerraformApplyRequest) (TerraformApplyResponse, error) {
	cancel := temporal.StartHeartbeat(ctx, temporal.HeartbeatTimeout)
	defer cancel()
	tfVersion, err := t.resolveVersion(request.TfVersion)
	if err != nil {
		return TerraformApplyResponse{}, err
	}

	planFile := request.PlanFile
	args := []command.Argument{DisableInputArg}
	args = append(args, request.Args...)

	envs, err := getEnvs(request.DynamicEnvs)
	if err != nil {
		return TerraformApplyResponse{}, err
	}
	t.addTerraformEnvs(envs, request.Path, tfVersion)

	applyRequest := &command.RunCommandRequest{
		RootPath:          request.Path,
		SubCommand:        command.NewSubCommand(command.TerraformApply).WithInput(planFile).WithUniqueArgs(args...),
		AdditionalEnvVars: envs,
		Version:           tfVersion,
	}
	out, err := t.runCommandWithOutputStream(ctx, request.JobID, applyRequest)

	if err != nil {
		activity.GetLogger(ctx).Error(out)
		return TerraformApplyResponse{}, wrapTerraformError(err, "running apply command")
	}

	return TerraformApplyResponse{}, nil
}

func (t *terraformActivities) runCommandWithOutputStream(ctx context.Context, jobID string, request *command.RunCommandRequest) (string, error) {
	reader, writer := io.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	var err error
	go func() {
		defer wg.Done()
		defer func() {
			if e := writer.Close(); e != nil {
				activity.GetLogger(ctx).Error("closing pipe writer", key.ErrKey, e)
			}
		}()
		err = t.TerraformClient.RunCommand(ctx, request, command.RunOptions{
			StdOut: writer,
			StdErr: writer,
		})
	}()

	s := bufio.NewScanner(reader)

	buf := []byte{}
	s.Buffer(buf, bufioScannerBufferSize)

	var output strings.Builder
	ch := t.StreamHandler.RegisterJob(jobID)
	for s.Scan() {
		_, err := output.WriteString(s.Text())
		if err != nil {
			activity.GetLogger(ctx).Warn("unable to write tf output to buffer")
		}
		ch <- s.Text()
	}

	close(ch)

	wg.Wait()

	return output.String(), err
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

func (t *terraformActivities) addTerraformEnvs(envs map[string]string, path string, tfVersion *version.Version) {
	envs[TFInAutomation] = TFInAutomationVal
	envs[AtlantisTerraformVersion] = tfVersion.String()
	envs[Dir] = path
	envs[TFPluginCacheDir] = t.CacheDir
	// This is not a long-term fix. Eventually the underlying functionality in terraform will be changed.
	// See https://developer.hashicorp.com/terraform/cli/config/config-file#allowing-the-provider-plugin-cache-to-break-the-dependency-lock-file
	// and https://github.com/hashicorp/terraform/issues/32205 for discussions and context.
	envs[PluginCacheMayBreakDependencyLockFile] = "true"
}
