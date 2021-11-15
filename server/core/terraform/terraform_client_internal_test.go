package terraform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	version "github.com/hashicorp/go-version"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/feature"
	fmocks "github.com/runatlantis/atlantis/server/feature/mocks"
	handlermocks "github.com/runatlantis/atlantis/server/handlers/mocks"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

// Test that it executes with the expected env vars.
func TestDefaultClient_RunCommandWithVersion_EnvVars(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "echo",
		usePluginCache:          true,
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}

	args := []string{
		"TF_IN_AUTOMATION=$TF_IN_AUTOMATION",
		"TF_PLUGIN_CACHE_DIR=$TF_PLUGIN_CACHE_DIR",
		"WORKSPACE=$WORKSPACE",
		"ATLANTIS_TERRAFORM_VERSION=$ATLANTIS_TERRAFORM_VERSION",
		"DIR=$DIR",
	}
	customEnvVars := map[string]string{}
	out, err := client.RunCommandWithVersion(ctx, tmp, args, customEnvVars, nil, "workspace")
	Ok(t, err)
	exp := fmt.Sprintf("TF_IN_AUTOMATION=true TF_PLUGIN_CACHE_DIR=%s WORKSPACE=workspace ATLANTIS_TERRAFORM_VERSION=0.11.11 DIR=%s\n", tmp, tmp)
	Equals(t, exp, out)
}

// Test that it returns an error on error.
func TestDefaultClient_RunCommandWithVersion_Error(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "echo",
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}

	args := []string{
		"dying",
		"&&",
		"exit",
		"1",
	}
	out, err := client.RunCommandWithVersion(ctx, tmp, args, map[string]string{}, nil, "workspace")
	ErrEquals(t, fmt.Sprintf(`running "echo dying && exit 1" in %q: exit status 1`, tmp), err)
	// Test that we still get our output.
	Equals(t, "dying\n", out)
}

func TestDefaultClient_RunCommandAsync_Success(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "echo",
		usePluginCache:          true,
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}

	args := []string{
		"TF_IN_AUTOMATION=$TF_IN_AUTOMATION",
		"TF_PLUGIN_CACHE_DIR=$TF_PLUGIN_CACHE_DIR",
		"WORKSPACE=$WORKSPACE",
		"ATLANTIS_TERRAFORM_VERSION=$ATLANTIS_TERRAFORM_VERSION",
		"DIR=$DIR",
	}
	outCh := client.RunCommandAsync(ctx, tmp, args, map[string]string{}, nil, "workspace")

	out, err := waitCh(outCh)
	Ok(t, err)
	exp := fmt.Sprintf("TF_IN_AUTOMATION=true TF_PLUGIN_CACHE_DIR=%s WORKSPACE=workspace ATLANTIS_TERRAFORM_VERSION=0.11.11 DIR=%s", tmp, tmp)
	Equals(t, exp, out)
}

func TestDefaultClient_RunCommandAsync_BigOutput(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "cat",
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}
	filename := filepath.Join(tmp, "data")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	Ok(t, err)

	var exp string
	for i := 0; i < 1024; i++ {
		s := strings.Repeat("0", 10) + "\n"
		exp += s
		_, err = f.WriteString(s)
		Ok(t, err)
	}
	outCh := client.RunCommandAsync(ctx, tmp, []string{filename}, map[string]string{}, nil, "workspace")

	out, err := waitCh(outCh)
	Ok(t, err)
	Equals(t, strings.TrimRight(exp, "\n"), out)
}

func TestDefaultClient_RunCommandAsync_StderrOutput(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "echo",
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}
	outCh := client.RunCommandAsync(ctx, tmp, []string{"stderr", ">&2"}, map[string]string{}, nil, "workspace")

	out, err := waitCh(outCh)
	Ok(t, err)
	Equals(t, "stderr", out)
}

func TestDefaultClient_RunCommandAsync_ExitOne(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "echo",
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}
	outCh := client.RunCommandAsync(ctx, tmp, []string{"dying", "&&", "exit", "1"}, map[string]string{}, nil, "workspace")

	out, err := waitCh(outCh)
	ErrEquals(t, fmt.Sprintf(`running "echo dying && exit 1" in %q: exit status 1`, tmp), err)
	// Test that we still get our output.
	Equals(t, "dying", out)
}

func TestDefaultClient_RunCommandAsync_Input(t *testing.T) {
	v, err := version.NewVersion("0.11.11")
	Ok(t, err)
	tmp, cleanup := TempDir(t)
	logger := logging.NewNoopLogger(t)
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()

	ctx := models.ProjectCommandContext{
		Log:                logger,
		Workspace:          "default",
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		EscapedCommentArgs: []string{"comment", "args"},
		ProjectName:        "projectname",
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	defer cleanup()
	allocator := fmocks.NewMockAllocator()
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	client := &DefaultClient{
		defaultVersion:          v,
		terraformPluginCacheDir: tmp,
		overrideTF:              "read",
		featureAllocator:        allocator,
		projectCmdOutputHandler: projectCmdOutputHandler,
	}

	inCh := make(chan string)

	outCh := client.RunCommandAsyncWithInput(ctx, tmp, []string{"a", "&&", "echo", "$a"}, map[string]string{}, nil, "workspace", inCh)
	inCh <- "echo me\n"

	out, err := waitCh(outCh)
	Ok(t, err)
	Equals(t, "echo me", out)
}

func waitCh(ch <-chan Line) (string, error) {
	var ls []string
	for line := range ch {
		if line.Err != nil {
			return strings.Join(ls, "\n"), line.Err
		}
		ls = append(ls, line.Line)
	}
	return strings.Join(ls, "\n"), nil
}
