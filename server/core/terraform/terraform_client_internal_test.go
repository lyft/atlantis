package terraform

import (
	"fmt"
	"os/exec"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/core/terraform/mocks"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/feature"
	fmocks "github.com/runatlantis/atlantis/server/feature/mocks"
	handlermocks "github.com/runatlantis/atlantis/server/handlers/mocks"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

// Test that it executes successfully
func TestDefaultClient_Synchronous_RunCommandWithVersion(t *testing.T) {
	path := "some/path"
	args := []string{
		"ARG1=$ARG1",
	}
	workspace := "workspace"
	logger := logging.NewNoopLogger(t)
	echoCommand := exec.Command("sh", "-c", "echo hello")

	ctx := models.ProjectCommandContext{
		Log: logger,
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	mockBuilder := mocks.NewMockcommandBuilder()
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()
	asyncClient := &AsyncClient{
		projectCmdOutputHandler: projectCmdOutputHandler,
		commandBuilder:          mockBuilder,
	}
	allocator := fmocks.NewMockAllocator()

	client := &DefaultClient{
		commandBuilder:   mockBuilder,
		AsyncClient:      asyncClient,
		featureAllocator: allocator,
	}
	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	When(mockBuilder.Build(nil, workspace, path, args)).ThenReturn(echoCommand, nil)

	customEnvVars := map[string]string{}
	out, err := client.RunCommandWithVersion(ctx, path, args, customEnvVars, nil, workspace)
	Ok(t, err)
	Equals(t, "hello\n", out)
}

// Test that it returns an error on error.
func TestDefaultClient_Synchronous_RunCommandWithVersion_Error(t *testing.T) {
	path := "some/path"
	args := []string{
		"ARG1=$ARG1",
	}
	workspace := "workspace"
	logger := logging.NewNoopLogger(t)
	echoCommand := exec.Command("sh", "-c", "echo dying && exit 1")

	ctx := models.ProjectCommandContext{
		Log: logger,
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}
	mockBuilder := mocks.NewMockcommandBuilder()
	projectCmdOutputHandler := handlermocks.NewMockProjectCommandOutputHandler()
	asyncClient := &AsyncClient{
		projectCmdOutputHandler: projectCmdOutputHandler,
		commandBuilder:          mockBuilder,
	}
	allocator := fmocks.NewMockAllocator()

	client := &DefaultClient{
		commandBuilder:   mockBuilder,
		AsyncClient:      asyncClient,
		featureAllocator: allocator,
	}

	When(allocator.ShouldAllocate(feature.LogStreaming, "owner/repo")).ThenReturn(false, nil)
	When(mockBuilder.Build(nil, workspace, path, args)).ThenReturn(echoCommand, nil)
	out, err := client.RunCommandWithVersion(ctx, path, args, map[string]string{}, nil, workspace)
	ErrEquals(t, fmt.Sprintf(`running "/bin/sh -c echo dying && exit 1" in %q: exit status 1`, path), err)
	// Test that we still get our output.
	Equals(t, "dying\n", out)
}
