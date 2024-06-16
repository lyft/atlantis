package activities

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"

	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/file"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

type testCredsRefresher struct {
	called                 bool
	expectedInstallationID int64
	t                      *testing.T
}

func (t *testCredsRefresher) Refresh(ctx context.Context, installationID int64) error {
	assert.Equal(t.t, t.expectedInstallationID, installationID)
	t.called = true
	return nil
}

type testStreamHandler struct {
	received      []string
	expectedJobID string
	t             *testing.T
	called        bool
	wg            sync.WaitGroup
}

func (t *testStreamHandler) RegisterJob(id string) chan string {
	ch := make(chan string)
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for s := range ch {
			t.received = append(t.received, s)
		}
		t.called = true
	}()
	return ch
}

func (t *testStreamHandler) Wait() {
	t.wg.Wait()
}

type multiCallTfClient struct {
	clients []*testTfClient

	count int
}

func (t *multiCallTfClient) RunCommand(ctx context.Context, request *command.RunCommandRequest, options ...command.RunOptions) error {
	if t.count >= len(t.clients) {
		return fmt.Errorf("expected less calls to RunCommand")
	}
	_ = t.clients[t.count].RunCommand(ctx, request, options...)

	t.count++

	return nil
}

func (t *multiCallTfClient) AssertExpectations() error {
	if t.count != len(t.clients) {
		return fmt.Errorf("expected %d calls but got %d", len(t.clients), t.count)
	}
	return nil
}

type testTfClient struct {
	t             *testing.T
	jobID         string
	path          string
	cmd           *command.SubCommand
	customEnvVars map[string]string
	version       *version.Version
	resp          string

	expectedError error
}

func (t *testTfClient) RunCommand(ctx context.Context, request *command.RunCommandRequest, options ...command.RunOptions) error {
	assert.Equal(t.t, t.path, request.RootPath)
	assert.Equal(t.t, t.cmd, request.SubCommand)
	assert.Equal(t.t, t.customEnvVars, request.AdditionalEnvVars)
	assert.Equal(t.t, t.version, request.Version)

	for _, o := range options {
		if o.StdOut != nil {
			_, err := o.StdOut.Write([]byte(t.resp))
			assert.NoError(t.t, err)
		}
	}

	return t.expectedError
}

func TestTerraformInit_RequestValidation(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		},
	}
	defaultVersion := "1.0.2"

	cases := []struct {
		RequestVersion  string
		ExpectedVersion string
		RequestArgs     []command.Argument
		ExpectedEnvs    map[string]string
		DynamicEnvs     []EnvVar
		ExpectedArgs    []command.Argument
	}{
		{
			//testing
			RequestVersion:  "0.12.0",
			ExpectedVersion: "0.12.0",

			//defaults
			ExpectedArgs: defaultArgs,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "0.12.0",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			//testing
			ExpectedArgs: []command.Argument{
				{
					Key:   "input",
					Value: "true",
				},
			},
			RequestArgs: []command.Argument{
				{
					Key:   "input",
					Value: "true",
				},
			},

			// defaults
			ExpectedVersion: defaultVersion,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			// testing
			DynamicEnvs: []EnvVar{
				{
					Name:  "env2",
					Value: "val2",
				},
			},
			ExpectedEnvs: map[string]string{
				"env2":                       "val2",
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},

			// defaults
			ExpectedVersion: defaultVersion,
			ExpectedArgs:    defaultArgs,
		},
	}

	for _, c := range cases {
		t.Run("request param takes precedence", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			path := "some/path"
			jobID := "1234"

			expectedVersion, err := version.NewVersion(c.ExpectedVersion)
			assert.Nil(t, err)

			testTfClient := &testTfClient{
				t:             t,
				jobID:         jobID,
				path:          path,
				cmd:           command.NewSubCommand(command.TerraformInit).WithUniqueArgs(c.ExpectedArgs...),
				customEnvVars: c.ExpectedEnvs,
				version:       expectedVersion,
				resp:          "",
			}

			req := TerraformInitRequest{
				DynamicEnvs: c.DynamicEnvs,
				JobID:       jobID,
				Path:        path,
				TfVersion:   c.RequestVersion,
				Args:        c.RequestArgs,
			}

			credsRefresher := &testCredsRefresher{
				expectedInstallationID: 1235,
				t:                      t,
			}

			tfActivity := NewTerraformActivities(
				testTfClient,
				expectedVersion,
				&testStreamHandler{
					t: t,
				},
				credsRefresher,
				&file.RWLock{},
				&mockWriter{},
				"some/dir",
				1235)
			env.RegisterActivity(tfActivity)

			_, err = env.ExecuteActivity(tfActivity.TerraformInit, req)
			assert.NoError(t, err)

			assert.True(t, credsRefresher.called)
		})
	}
}

func TestTerraformInit_StreamsOutput(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		},
	}
	defaultVersion := "1.0.2"

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	path := "some/path"
	jobID := "1234"

	expectedMsgs := []string{"msg1", "msg2"}
	expectedMsgStr := strings.Join(expectedMsgs, "\n")

	expectedVersion, err := version.NewVersion(defaultVersion)
	assert.NoError(t, err)

	testTfClient := &testTfClient{
		t:     t,
		jobID: jobID,
		path:  path,
		cmd:   command.NewSubCommand(command.TerraformInit).WithUniqueArgs(defaultArgs...),
		customEnvVars: map[string]string{
			"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
			"DIR":                        "some/path",
			"TF_IN_AUTOMATION":           "true",
			"TF_PLUGIN_CACHE_DIR":        "some/dir",
			"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
		},
		version: expectedVersion,
		resp:    expectedMsgStr,
	}

	req := TerraformInitRequest{
		JobID: jobID,
		Path:  path,
	}

	streamHandler := &testStreamHandler{
		t:             t,
		received:      expectedMsgs,
		expectedJobID: jobID,
	}

	credsRefresher := &testCredsRefresher{
		expectedInstallationID: 1235,
		t:                      t,
	}

	tfActivity := NewTerraformActivities(testTfClient, expectedVersion, streamHandler, credsRefresher, &file.RWLock{}, &mockWriter{}, "some/dir", 1235)
	env.RegisterActivity(tfActivity)

	_, err = env.ExecuteActivity(tfActivity.TerraformInit, req)
	assert.NoError(t, err)

	// wait before we check called value otherwise we might race
	streamHandler.Wait()
	assert.True(t, streamHandler.called)
}

func TestTerraformPlan_RequestValidation(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		}, {
			Key:   "refresh",
			Value: "true",
		}, {
			Key:   "out",
			Value: "some/path/output.tfplan",
		}}
	defaultVersion := "1.0.2"

	cases := []struct {
		RequestVersion  string
		ExpectedVersion string
		RequestArgs     []command.Argument
		ExpectedArgs    []command.Argument
		ExpectedFlags   []command.Flag
		PlanMode        *terraform.PlanMode
		WorkflowMode    terraform.WorkflowMode
		ExpectedEnvs    map[string]string
		DynamicEnvs     []EnvVar
	}{
		{
			//testing
			WorkflowMode:    terraform.PR,
			RequestVersion:  "0.12.0",
			ExpectedVersion: "0.12.0",

			//default
			ExpectedArgs: defaultArgs,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "0.12.0",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			//testing
			WorkflowMode: terraform.PR,
			ExpectedArgs: []command.Argument{
				{
					Key:   "input",
					Value: "true",
				}, {
					Key:   "refresh",
					Value: "true",
				}, {
					Key:   "out",
					Value: "some/path/output.tfplan",
				}},
			RequestArgs: []command.Argument{
				{
					Key:   "input",
					Value: "true",
				},
			},

			// default
			ExpectedVersion: defaultVersion,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			// testing
			PlanMode:     terraform.NewDestroyPlanMode(),
			WorkflowMode: terraform.PR,
			ExpectedFlags: []command.Flag{
				{
					Value: "destroy",
				},
			},

			// default
			ExpectedArgs:    defaultArgs,
			ExpectedVersion: defaultVersion,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			// testing
			WorkflowMode: terraform.PR,
			DynamicEnvs: []EnvVar{
				{
					Name:  "env2",
					Value: "val2",
				},
			},
			ExpectedEnvs: map[string]string{
				"env2":                       "val2",
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},

			// default
			ExpectedArgs:    defaultArgs,
			ExpectedVersion: defaultVersion,
		},
	}

	for _, c := range cases {
		t.Run("request param takes precedence", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			path := "some/path"
			jobID := "1234"

			expectedVersion, err := version.NewVersion(c.ExpectedVersion)
			assert.Nil(t, err)

			testTfClient := multiCallTfClient{
				clients: []*testTfClient{
					{
						t:             t,
						jobID:         jobID,
						path:          path,
						cmd:           command.NewSubCommand(command.TerraformPlan).WithUniqueArgs(c.ExpectedArgs...).WithFlags(c.ExpectedFlags...),
						customEnvVars: c.ExpectedEnvs,
						version:       expectedVersion,
						resp:          "",
					},
					{
						t:             t,
						jobID:         jobID,
						path:          path,
						cmd:           command.NewSubCommand(command.TerraformShow).WithFlags(command.Flag{Value: "json"}).WithInput("some/path/output.tfplan"),
						customEnvVars: c.ExpectedEnvs,
						version:       expectedVersion,
						resp:          "{}",
					},
				},
			}

			req := TerraformPlanRequest{
				DynamicEnvs:  c.DynamicEnvs,
				JobID:        jobID,
				Path:         path,
				TfVersion:    c.RequestVersion,
				Args:         c.RequestArgs,
				PlanMode:     c.PlanMode,
				WorkflowMode: c.WorkflowMode,
			}

			credsRefresher := &testCredsRefresher{}
			fileWriter := &mockWriter{
				t:            t,
				expectedName: "some/path/output.json",
			}

			tfActivity := NewTerraformActivities(&testTfClient, expectedVersion, &testStreamHandler{
				t: t,
			}, credsRefresher, &file.RWLock{}, fileWriter, "some/dir", 0)
			env.RegisterActivity(tfActivity)

			_, err = env.ExecuteActivity(tfActivity.TerraformPlan, req)
			assert.NoError(t, err)
			assert.NoError(t, testTfClient.AssertExpectations())
		})
	}
}

func TestTerraformPlan_ReturnsResponse(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		}, {
			Key:   "refresh",
			Value: "true",
		}, {
			Key:   "out",
			Value: "some/path/output.tfplan",
		}}
	defaultVersion := "1.0.2"

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	path := "some/path"
	jobID := "1234"

	expectedMsgs := []string{"msg1", "msg2"}
	expectedMsgStr := strings.Join(expectedMsgs, "\n")

	expectedVersion, err := version.NewVersion(defaultVersion)
	assert.Nil(t, err)

	testTfClient := multiCallTfClient{
		clients: []*testTfClient{
			{
				t:     t,
				jobID: jobID,
				path:  path,
				cmd:   command.NewSubCommand(command.TerraformPlan).WithUniqueArgs(defaultArgs...),
				customEnvVars: map[string]string{
					"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
					"DIR":                        "some/path",
					"TF_IN_AUTOMATION":           "true",
					"TF_PLUGIN_CACHE_DIR":        "some/dir",
					"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
				},
				version: expectedVersion,
				resp:    expectedMsgStr,
			},
			{
				t:     t,
				jobID: jobID,
				path:  path,
				cmd:   command.NewSubCommand(command.TerraformShow).WithFlags(command.Flag{Value: "json"}).WithInput("some/path/output.tfplan"),
				customEnvVars: map[string]string{
					"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
					"DIR":                        "some/path",
					"TF_IN_AUTOMATION":           "true",
					"TF_PLUGIN_CACHE_DIR":        "some/dir",
					"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
				},
				version: expectedVersion,
				resp:    "{\"format_version\": \"1.0\",\"resource_changes\":[{\"change\":{\"actions\":[\"update\"]},\"address\":\"type.resource\"}]}",
			},
		},
	}

	req := TerraformPlanRequest{
		JobID: jobID,
		Path:  path,
	}

	streamHandler := &testStreamHandler{
		t:             t,
		received:      expectedMsgs,
		expectedJobID: jobID,
	}

	credsRefresher := &testCredsRefresher{}

	tfActivity := NewTerraformActivities(&testTfClient, expectedVersion, streamHandler, credsRefresher, &file.RWLock{}, &mockWriter{}, "some/dir", 0)

	env.RegisterActivity(tfActivity)

	result, err := env.ExecuteActivity(tfActivity.TerraformPlan, req)
	assert.NoError(t, err)
	assert.NoError(t, testTfClient.AssertExpectations())

	var resp TerraformPlanResponse
	assert.NoError(t, result.Get(&resp))

	assert.Equal(t, TerraformPlanResponse{
		PlanFile: "some/path/output.tfplan",
		Summary: terraform.PlanSummary{
			Updates: []terraform.ResourceSummary{
				{
					Address: "type.resource",
				},
			},
		},
	}, resp)

	// wait before we check called value otherwise we might race
	streamHandler.Wait()
	assert.True(t, streamHandler.called)
}

func TestTerraformApply_RequestValidation(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		},
	}
	defaultVersion := "1.0.2"

	cases := []struct {
		RequestVersion  string
		ExpectedVersion string
		RequestArgs     []command.Argument
		ExpectedArgs    []command.Argument
		ExpectedEnvs    map[string]string
		DynamicEnvs     []EnvVar
	}{
		{
			//testing
			RequestVersion:  "0.12.0",
			ExpectedVersion: "0.12.0",

			//default
			ExpectedArgs: defaultArgs,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "0.12.0",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			//testing
			ExpectedArgs: []command.Argument{
				{
					Key:   "input",
					Value: "false",
				}},
			RequestArgs: []command.Argument{
				{
					Key:   "input",
					Value: "false",
				},
			},
			//default
			ExpectedVersion: defaultVersion,
			ExpectedEnvs: map[string]string{
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},
		},
		{
			//testing
			DynamicEnvs: []EnvVar{
				{
					Name:  "env2",
					Value: "val2",
				},
			},
			ExpectedEnvs: map[string]string{
				"env2":                       "val2",
				"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
				"DIR":                        "some/path",
				"TF_IN_AUTOMATION":           "true",
				"TF_PLUGIN_CACHE_DIR":        "some/dir",
				"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
			},

			//default
			ExpectedVersion: defaultVersion,
			ExpectedArgs:    defaultArgs,
		},
	}

	for _, c := range cases {
		t.Run("request param takes precedence", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			path := "some/path"
			jobID := "1234"

			expectedVersion, err := version.NewVersion(c.ExpectedVersion)
			assert.Nil(t, err)

			testClient := &testTfClient{
				t:             t,
				jobID:         jobID,
				path:          path,
				cmd:           command.NewSubCommand(command.TerraformApply).WithUniqueArgs(c.ExpectedArgs...).WithInput("some/path/output.tfplan"),
				customEnvVars: c.ExpectedEnvs,
				version:       expectedVersion,
				resp:          "",
			}

			req := TerraformApplyRequest{
				DynamicEnvs: c.DynamicEnvs,
				JobID:       jobID,
				Path:        path,
				TfVersion:   c.RequestVersion,
				Args:        c.RequestArgs,
				PlanFile:    "some/path/output.tfplan",
			}

			tfActivity := NewTerraformActivities(testClient, expectedVersion, &testStreamHandler{
				t: t,
			}, &testCredsRefresher{}, &file.RWLock{}, &mockWriter{}, "some/dir", 0)
			env.RegisterActivity(tfActivity)

			_, err = env.ExecuteActivity(tfActivity.TerraformApply, req)
			assert.NoError(t, err)
		})
	}
}

func TestTerraformApply_StreamsOutput(t *testing.T) {
	defaultArgs := []command.Argument{
		{
			Key:   "input",
			Value: "false",
		},
	}
	defaultVersion := "1.0.2"

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	path := "some/path"
	jobID := "1234"

	expectedMsgs := []string{"msg1", "msg2"}
	expectedMsgStr := strings.Join(expectedMsgs, "\n")

	expectedVersion, err := version.NewVersion(defaultVersion)
	assert.NoError(t, err)

	testTfClient := &testTfClient{
		t:     t,
		jobID: jobID,
		path:  path,
		cmd:   command.NewSubCommand(command.TerraformApply).WithUniqueArgs(defaultArgs...).WithInput("some/path/output.tfplan"),
		customEnvVars: map[string]string{
			"ATLANTIS_TERRAFORM_VERSION": "1.0.2",
			"DIR":                        "some/path",
			"TF_IN_AUTOMATION":           "true",
			"TF_PLUGIN_CACHE_DIR":        "some/dir",
			"TF_PLUGIN_CACHE_MAY_BREAK_DEPENDENCY_LOCK_FILE": "true",
		},
		version: expectedVersion,
		resp:    expectedMsgStr,
	}

	req := TerraformApplyRequest{
		JobID:    jobID,
		Path:     path,
		PlanFile: "some/path/output.tfplan",
	}

	streamHandler := &testStreamHandler{
		t:             t,
		received:      expectedMsgs,
		expectedJobID: jobID,
	}

	tfActivity := NewTerraformActivities(testTfClient, expectedVersion, streamHandler, &testCredsRefresher{}, &file.RWLock{}, &mockWriter{}, "some/dir", 0)
	env.RegisterActivity(tfActivity)

	_, err = env.ExecuteActivity(tfActivity.TerraformApply, req)
	assert.NoError(t, err)

	// wait before we check called value otherwise we might race
	streamHandler.Wait()
	assert.True(t, streamHandler.called)
}

type mockWriter struct {
	t            *testing.T
	expectedName string
}

func (m *mockWriter) Write(name string, _ []byte) error {
	assert.Equal(m.t, m.expectedName, name)
	return nil
}
