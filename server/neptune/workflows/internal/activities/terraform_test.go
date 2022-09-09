package activities_test

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/terraform"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/activities"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

type testTfClient struct {
	t             *testing.T
	ctx           context.Context
	jobID         string
	path          string
	args          []string
	customEnvVars map[string]string
	version       *version.Version

	resp []terraform.Line
}

func (t *testTfClient) RunCommand(ctx context.Context, jobID string, path string, args []string, customEnvVars map[string]string, v *version.Version) <-chan terraform.Line {
	assert.Equal(t.t, jobID, t.jobID)
	assert.Equal(t.t, path, t.path)
	assert.Equal(t.t, args, t.args)
	assert.Equal(t.t, customEnvVars, t.customEnvVars)
	assert.Equal(t.t, v, t.version)

	ch := make(chan terraform.Line)
	go func(ch chan terraform.Line) {
		defer close(ch)
		for _, line := range t.resp {
			ch <- line
		}
	}(ch)

	return ch

}
func TestTerraformInit_TfVersionInRequestTakesPrecedence(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	ctx := context.Background()
	path := "some/path"
	jobID := "1234"
	defVersion := "1.0.2"
	reqVersion := "0.12.0"

	defaultTfVersion, err := version.NewVersion(defVersion)
	assert.Nil(t, err)

	reqTfVersion, err := version.NewVersion(reqVersion)
	assert.Nil(t, err)

	testTfClient := testTfClient{
		t:             t,
		ctx:           ctx,
		jobID:         jobID,
		path:          path,
		args:          []string{"init", "-input=false"},
		customEnvVars: map[string]string{},
		version:       reqTfVersion,
		resp:          []terraform.Line{},
	}

	req := activities.TerraformInitRequest{
		Step: job.Step{
			StepName: "init",
		},
		Envs:      map[string]string{},
		JobID:     jobID,
		Path:      path,
		TfVersion: reqVersion,
	}

	tfActivity := activities.NewTerraformActivities(&testTfClient, defaultTfVersion)
	env.RegisterActivity(tfActivity)

	_, err = env.ExecuteActivity(tfActivity.TerraformInit, req)
	assert.NoError(t, err)
}

func TestTerraformInit_ExtraArgsTakesPrecedenceOverCommandArgs(t *testing.T) {
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	ctx := context.Background()
	path := "some/path"
	jobID := "1234"
	defVersion := "1.0.2"
	reqVersion := "0.12.0"

	defaultTfVersion, err := version.NewVersion(defVersion)
	assert.Nil(t, err)

	reqTfVersion, err := version.NewVersion(reqVersion)
	assert.Nil(t, err)

	testTfClient := testTfClient{
		t:             t,
		ctx:           ctx,
		jobID:         jobID,
		path:          path,
		args:          []string{"init", "-input=true"},
		customEnvVars: map[string]string{},
		version:       reqTfVersion,
		resp:          []terraform.Line{},
	}

	req := activities.TerraformInitRequest{
		Step: job.Step{
			StepName:  "init",
			ExtraArgs: []string{"-input=true"},
		},
		Envs:      map[string]string{},
		JobID:     jobID,
		Path:      path,
		TfVersion: reqVersion,
	}

	tfActivity := activities.NewTerraformActivities(&testTfClient, defaultTfVersion)
	env.RegisterActivity(tfActivity)

	_, err = env.ExecuteActivity(tfActivity.TerraformInit, req)
	assert.NoError(t, err)
}
