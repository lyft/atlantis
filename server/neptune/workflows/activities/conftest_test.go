package activities

import (
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
	"strings"
	"testing"
)

func TestConftest_RequestValidation(t *testing.T) {
	version, err := version.NewVersion("0.20.0")
	assert.Nil(t, err)

	expectedArgs := []command.Argument{
		{
			Key:   "p",
			Value: "path/one",
		},
		{
			Key:   "p",
			Value: "path/two",
		},
	}

	expectedFlags := []command.Flag{
		{
			Value: "-no-color",
		},
	}

	cases := []struct {
		RequestArgs   []command.Argument
		ExpectedArgs  []command.Argument
		ExpectedFlags []command.Flag
		ExpectedEnvs  map[string]string
		DynamicEnvs   []EnvVar
	}{
		{
			//default
			ExpectedArgs:  expectedArgs,
			ExpectedEnvs:  map[string]string{},
			ExpectedFlags: expectedFlags,
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
				"env2": "val2",
			},
			ExpectedArgs:  expectedArgs,
			ExpectedFlags: expectedFlags,
		},
	}

	for _, c := range cases {
		t.Run("request param takes precedence", func(t *testing.T) {
			ts := testsuite.WorkflowTestSuite{}
			env := ts.NewTestActivityEnvironment()

			path := "some/path"
			jobID := "1234"

			testClient := &testTfClient{
				t:             t,
				jobID:         jobID,
				path:          path,
				cmd:           command.NewSubCommand(command.ConftestTest).WithArgs(c.ExpectedArgs...).WithInput("some/path/output.json").WithFlags(c.ExpectedFlags...),
				customEnvVars: c.ExpectedEnvs,
				version:       version,
				resp:          "",
			}

			req := ConftestRequest{
				DynamicEnvs: c.DynamicEnvs,
				JobID:       jobID,
				Path:        path,
				Args:        c.RequestArgs,
				ShowFile:    "some/path/output.json",
			}

			policySets := []PolicySet{
				{
					Name:  "policy1",
					Paths: []string{"path/one", "path/two"},
				},
			}
			activity := conftestActivity{
				DefaultConftestVersion: version,
				ConftestClient:         testClient,
				StreamHandler:          &testStreamHandler{t: t},
				Policies:               policySets,
				FileValidator:          &mockStat{t: t, expectedName: "some/path/output.json"},
			}
			env.RegisterActivity(activity.Conftest)

			_, err := env.ExecuteActivity(activity.Conftest, req)
			assert.NoError(t, err)
		})
	}
}

func TestConftest_StreamsOutput(t *testing.T) {
	expectedArgs := []command.Argument{
		{
			Key:   "p",
			Value: "path/one",
		},
		{
			Key:   "p",
			Value: "path/two",
		},
	}

	expectedFlags := []command.Flag{
		{
			Value: "-no-color",
		},
	}

	version, err := version.NewVersion("0.20.0")
	assert.Nil(t, err)

	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()

	path := "some/path"
	jobID := "1234"

	expectedMsgs := []string{"msg1", "msg2"}
	expectedMsgStr := strings.Join(expectedMsgs, "\n")

	testClient := &testTfClient{
		t:             t,
		jobID:         jobID,
		path:          path,
		cmd:           command.NewSubCommand(command.ConftestTest).WithArgs(expectedArgs...).WithInput("some/path/output.json").WithFlags(expectedFlags...),
		customEnvVars: map[string]string{},
		version:       version,
		resp:          expectedMsgStr,
	}

	req := ConftestRequest{
		JobID:    jobID,
		Path:     path,
		ShowFile: "some/path/output.json",
	}

	streamHandler := &testStreamHandler{
		t:             t,
		received:      expectedMsgs,
		expectedJobID: jobID,
	}

	policySets := []PolicySet{
		{
			Name:  "policy1",
			Paths: []string{"path/one", "path/two"},
		},
	}
	activity := conftestActivity{
		DefaultConftestVersion: version,
		ConftestClient:         testClient,
		StreamHandler:          streamHandler,
		Policies:               policySets,
		FileValidator:          &mockStat{t: t, expectedName: "some/path/output.json"},
	}
	env.RegisterActivity(activity.Conftest)
	_, err = env.ExecuteActivity(activity.Conftest, req)
	assert.NoError(t, err)

	// wait before we check called value otherwise we might race
	streamHandler.Wait()
	assert.True(t, streamHandler.called)
}

func TestConftest_ShowFileMissing(t *testing.T) {
	req := ConftestRequest{
		ShowFile: "some/path/output.json",
	}
	ts := testsuite.WorkflowTestSuite{}
	env := ts.NewTestActivityEnvironment()
	activity := conftestActivity{
		FileValidator: &mockStat{t: t, expectedName: "some/path/output.json", error: assert.AnError},
	}
	env.RegisterActivity(activity.Conftest)
	_, err := env.ExecuteActivity(activity.Conftest, req)
	assert.Error(t, err)
}

type mockStat struct {
	t            *testing.T
	expectedName string
	error        error
}

func (s *mockStat) Stat(name string) error {
	assert.Equal(s.t, s.expectedName, name)
	return s.error
}
