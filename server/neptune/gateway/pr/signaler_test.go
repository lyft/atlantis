package pr_test

import (
	"context"
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/pr"
	"github.com/runatlantis/atlantis/server/neptune/workflows"
	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/client"
	"testing"
)

func TestWorkflowSignaler_SignalWithStartWorkflow_Success(t *testing.T) {
	testWorkflow := valid.Workflow{
		Name:        "test-workflow",
		Plan:        valid.DefaultPlanStage,
		PolicyCheck: valid.DefaultPolicyCheckStage,
	}
	rootCfgs := []*valid.MergedProjectCfg{
		{
			Name:                "root1",
			RepoRelDir:          "some/path/`",
			Tags:                make(map[string]string),
			PullRequestWorkflow: testWorkflow,
		},
		{
			Name:                "root2",
			RepoRelDir:          "some/path/1",
			Tags:                make(map[string]string),
			PullRequestWorkflow: testWorkflow,
		},
	}
	testRepo := models.Repo{
		FullName:      "some/test",
		Owner:         "owner",
		DefaultBranch: "main",
	}
	prOptions := pr.Options{
		Number:            1,
		Revision:          "abc",
		Repo:              testRepo,
		InstallationToken: 100,
		Branch:            "test",
	}
	mockTemporalClient := &mockTemporalClient{
		t:                  t,
		expectedWorkflowID: "some/test||1",
		expectedRunID:      "456",
		expectedSignalName: workflows.PRTerraformRevisionSignalID,
		expectedSignalArg: workflows.PRNewRevisionSignalRequest{
			Revision: "abc",
			Roots:    buildRoots(rootCfgs),
			Repo: workflows.PRRepo{
				URL:           testRepo.CloneURL,
				FullName:      testRepo.FullName,
				Name:          testRepo.Name,
				Owner:         testRepo.Owner,
				DefaultBranch: testRepo.DefaultBranch,
				Credentials: workflows.PRAppCredentials{
					InstallationToken: prOptions.InstallationToken,
				},
			},
		},
		expectedOptions: client.StartWorkflowOptions{
			TaskQueue: workflows.PRTaskQueue,
			SearchAttributes: map[string]interface{}{
				"atlantis_repository": prOptions.Repo.FullName,
			},
		},
		expectedWorkflow: workflows.PR,
		expectedWorkflowArgs: workflows.PRRequest{
			RepoFullName: "some/test",
			PRNum:        1,
		},
		expectedErr: nil,
	}
	workflowSignaler := pr.WorkflowSignaler{TemporalClient: mockTemporalClient}
	run, err := workflowSignaler.SignalWithStartWorkflow(context.Background(), rootCfgs, prOptions)
	assert.NoError(t, err)
	assert.Equal(t, "123", run.GetID())
	assert.Equal(t, "456", run.GetRunID())

}

func TestWorkflowSignaler_SignalWithStartWorkflow_Failure(t *testing.T) {
	testRepo := models.Repo{
		FullName:      "some/test",
		Owner:         "owner",
		DefaultBranch: "main",
	}
	prOptions := pr.Options{
		Number:            1,
		Revision:          "abc",
		Repo:              testRepo,
		InstallationToken: 100,
		Branch:            "test",
	}
	mockTemporalClient := &mockTemporalClient{
		t:                  t,
		expectedWorkflowID: "some/test||1",
		expectedRunID:      "456",
		expectedSignalName: workflows.PRTerraformRevisionSignalID,
		expectedSignalArg: workflows.PRNewRevisionSignalRequest{
			Revision: "abc",
			Repo: workflows.PRRepo{
				URL:           testRepo.CloneURL,
				FullName:      testRepo.FullName,
				Name:          testRepo.Name,
				Owner:         testRepo.Owner,
				DefaultBranch: testRepo.DefaultBranch,
				Credentials: workflows.PRAppCredentials{
					InstallationToken: prOptions.InstallationToken,
				},
			},
		},
		expectedOptions: client.StartWorkflowOptions{
			TaskQueue: workflows.PRTaskQueue,
			SearchAttributes: map[string]interface{}{
				"atlantis_repository": prOptions.Repo.FullName,
			},
		},
		expectedWorkflow: workflows.PR,
		expectedWorkflowArgs: workflows.PRRequest{
			RepoFullName: "some/test",
			PRNum:        1,
		},
		expectedErr: assert.AnError,
	}
	workflowSignaler := pr.WorkflowSignaler{TemporalClient: mockTemporalClient}
	run, err := workflowSignaler.SignalWithStartWorkflow(context.Background(), []*valid.MergedProjectCfg{}, prOptions)
	assert.Error(t, err)
	assert.Nil(t, run)
}

func buildRoots(rootCfgs []*valid.MergedProjectCfg) []workflows.PRRoot {
	var roots []workflows.PRRoot
	for _, rootCfg := range rootCfgs {
		var tfVersion string
		if rootCfg.TerraformVersion != nil {
			tfVersion = rootCfg.TerraformVersion.String()
		}
		roots = append(roots, workflows.PRRoot{
			Name:        rootCfg.Name,
			RepoRelPath: rootCfg.RepoRelDir,
			TfVersion:   tfVersion,
			PlanMode:    workflows.PRNormalPlanMode,
			Plan:        workflows.PRJob{Steps: generateSteps(rootCfg.PullRequestWorkflow.Plan.Steps)},
			Validate:    workflows.PRJob{Steps: generateSteps(rootCfg.PullRequestWorkflow.PolicyCheck.Steps)},
		})
	}
	return roots
}

func generateSteps(steps []valid.Step) []workflows.PRStep {
	// NOTE: for deployment workflows, we won't support command level user requests for log level output verbosity
	var workflowSteps []workflows.PRStep
	for _, step := range steps {
		workflowSteps = append(workflowSteps, workflows.PRStep{
			StepName:    step.StepName,
			ExtraArgs:   step.ExtraArgs,
			RunCommand:  step.RunCommand,
			EnvVarName:  step.EnvVarName,
			EnvVarValue: step.EnvVarValue,
		})
	}
	return workflowSteps
}

type testRun struct{}

func (r testRun) GetID() string {
	return "123"
}

func (r testRun) GetRunID() string {
	return "456"
}

func (r testRun) Get(ctx context.Context, valuePtr interface{}) error {
	return nil
}

func (r testRun) GetWithOptions(ctx context.Context, valuePtr interface{}, options client.WorkflowRunGetOptions) error {
	return nil
}

type mockTemporalClient struct {
	t                    *testing.T
	expectedWorkflowID   string
	expectedRunID        string
	expectedSignalName   string
	expectedSignalArg    interface{}
	expectedOptions      client.StartWorkflowOptions
	expectedWorkflow     interface{}
	expectedWorkflowArgs interface{}
	expectedErr          error

	called bool
}

func (m mockTemporalClient) SignalWithStartWorkflow(ctx context.Context, workflowID string, signalName string, signalArg interface{}, options client.StartWorkflowOptions, workflow interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error) {
	m.called = true
	assert.Equal(m.t, m.expectedWorkflowID, workflowID)
	assert.Equal(m.t, m.expectedSignalName, signalName)
	assert.Equal(m.t, m.expectedSignalArg, signalArg)
	assert.Equal(m.t, m.expectedOptions, options)
	assert.IsType(m.t, m.expectedWorkflow, workflow)
	assert.Equal(m.t, []interface{}{m.expectedWorkflowArgs}, workflowArgs)
	if m.expectedErr != nil {
		return nil, m.expectedErr
	}
	return testRun{}, nil
}
