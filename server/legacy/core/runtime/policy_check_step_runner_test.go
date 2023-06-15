package runtime_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/core/runtime"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
)

const (
	expectedOutput = "output"
	executablePath = "some/path/conftest"
)

func buildTestPrjCtx(t *testing.T) command.ProjectContext {
	v, err := version.NewVersion("1.0")
	assert.NoError(t, err)
	return command.ProjectContext{
		Log: logging.NewNoopCtxLogger(t),
		BaseRepo: models.Repo{
			FullName: "owner/repo",
		},
		PolicySets: valid.PolicySets{
			Version:    v,
			PolicySets: []valid.PolicySet{},
		},
	}
}

func TestRun_Successful(t *testing.T) {
	prjCtx := buildTestPrjCtx(t)
	ensurer := &mockEnsurer{}
	executor := &mockExecutor{
		output: expectedOutput,
	}
	runner := &runtime.PolicyCheckStepRunner{
		VersionEnsurer: ensurer,
		Executor:       executor,
	}
	output, err := runner.Run(context.Background(), prjCtx, []string{}, executablePath, map[string]string{})
	assert.NoError(t, err)
	assert.Equal(t, output, expectedOutput)
	assert.True(t, ensurer.isCalled)
	assert.True(t, executor.isCalled)
}

func TestRun_EnsurerFailure(t *testing.T) {
	prjCtx := buildTestPrjCtx(t)
	ensurer := &mockEnsurer{
		err: assert.AnError,
	}
	executor := &mockExecutor{}
	runner := &runtime.PolicyCheckStepRunner{
		VersionEnsurer: ensurer,
		Executor:       executor,
	}
	output, err := runner.Run(context.Background(), prjCtx, []string{}, executablePath, map[string]string{})
	assert.Error(t, err)
	assert.Empty(t, output)
	assert.True(t, ensurer.isCalled)
	assert.False(t, executor.isCalled)
}

type mockExecutor struct {
	output   string
	err      error
	isCalled bool
}

func (t *mockExecutor) Run(_ context.Context, _ command.ProjectContext, _ string, _ map[string]string, _ string, _ []string) (string, error) {
	t.isCalled = true
	return t.output, t.err
}

type mockEnsurer struct {
	output   string
	err      error
	isCalled bool
}

func (t *mockEnsurer) EnsureExecutorVersion(_ logging.Logger, _ *version.Version) (string, error) {
	t.isCalled = true
	return t.output, t.err
}
