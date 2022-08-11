package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
)

func TestChecksEnabledApprovePoliciesCommandRunner_LogAndContinueWhenBuildPlanFails(t *testing.T) {

	underlyingRunner := testRunner{}

	comment := command.Comment{
		Name:        command.ApprovePolicies,
		Workspace:   "workspace",
		ProjectName: "project",
	}

	prjCmdBuilder := testPrjCmdBuilder{
		expectedResp: struct {
			prjCtxs []command.ProjectContext
			err     error
		}{
			prjCtxs: []command.ProjectContext{},
			err:     errors.New("error"),
		},
	}

	ctx := command.Context{
		Log:        logging.NewNoopCtxLogger(t),
		RequestCtx: context.TODO(),
	}

	cmdRunner := events.ChecksEnabledApprovePoliciesCommandRunner{
		Runner:            &underlyingRunner,
		FeatureAllocator:  testFeatureAllocator{isChecksEnabled: true},
		PrjCommandBuilder: &prjCmdBuilder,
	}

	cmdRunner.Run(&ctx, &comment)

	// Ensure the underlying runner was called
	if underlyingRunner.isExecuted == false {
		t.FailNow()
	}
}

func TestChecksEnabledApprovePoliciesCommandRunner_WriteOutputForFailingPolicyCheck(t *testing.T) {
	underlyingRunner := testRunner{}

	prjCmdRunner := testPolicyCheckCmdRunner{
		expectedResp: command.ProjectResult{
			PolicyCheckSuccess: nil,
			Failure:            "Policy Check Failed",
		},
	}

	prjCmdBuilder := testPrjCmdBuilder{
		expectedResp: struct {
			prjCtxs []command.ProjectContext
			err     error
		}{
			err: nil,
			prjCtxs: []command.ProjectContext{
				{
					CommandName: command.PolicyCheck,
					ProjectName: "project",
					Workspace:   "workspace",
				},
			},
		},
	}

	cmdRunner := events.ChecksEnabledApprovePoliciesCommandRunner{
		Runner:            &underlyingRunner,
		FeatureAllocator:  testFeatureAllocator{isChecksEnabled: true},
		PrjCommandBuilder: &prjCmdBuilder,
		PrjCommandRunner:  &prjCmdRunner,
	}

	comment := command.Comment{
		Name:        command.Plan,
		Workspace:   "workspace",
		ProjectName: "project",
	}

	ctx := command.Context{
		Log:        logging.NewNoopCtxLogger(t),
		RequestCtx: context.TODO(),
	}

	cmdRunner.Run(&ctx, &comment)

	// Assert ctx is populated with policy check output
	if comment.PolicyCheckOutput.GetOutputFor("project", "workspace") != "Policy Check Failed" {
		t.FailNow()
	}

	// Ensure the underlying runner was called
	if underlyingRunner.isExecuted == false {
		t.FailNow()
	}
}

func TestChecksEnabledApprovePoliciesCommandRunner_SkipPassingPolicyCheck(t *testing.T) {
	underlyingRunner := testRunner{}

	prjCmdRunner := testPolicyCheckCmdRunner{
		expectedResp: command.ProjectResult{
			PolicyCheckSuccess: &models.PolicyCheckSuccess{
				PolicyCheckOutput: "Policy Check Passed",
			},
		},
	}

	prjCmdBuilder := testPrjCmdBuilder{
		expectedResp: struct {
			prjCtxs []command.ProjectContext
			err     error
		}{
			err: nil,
			prjCtxs: []command.ProjectContext{
				{
					CommandName: command.PolicyCheck,
					ProjectName: "passing project",
					Workspace:   "passing workspace",
				},
			},
		},
	}

	cmdRunner := events.ChecksEnabledApprovePoliciesCommandRunner{
		Runner:            &underlyingRunner,
		FeatureAllocator:  testFeatureAllocator{isChecksEnabled: true},
		PrjCommandBuilder: &prjCmdBuilder,
		PrjCommandRunner:  &prjCmdRunner,
	}

	comment := command.Comment{
		Name:        command.Plan,
		Workspace:   "workspace",
		ProjectName: "project",
	}

	ctx := command.Context{
		Log:        logging.NewNoopCtxLogger(t),
		RequestCtx: context.TODO(),
	}

	cmdRunner.Run(&ctx, &comment)

	// Assert ctx is populated with policy check output
	if comment.PolicyCheckOutput.GetOutputFor("project", "workspace") != "" {
		t.FailNow()
	}

	// Ensure the underlying runner was called
	if underlyingRunner.isExecuted == false {
		t.FailNow()
	}
}

type testPolicyCheckCmdRunner struct {
	expectedResp command.ProjectResult
}

func (t *testPolicyCheckCmdRunner) PolicyCheck(ctx command.ProjectContext) command.ProjectResult {
	return t.expectedResp
}

type testRunner struct {
	isExecuted bool
}

func (t *testRunner) Run(ctx *command.Context, cmd *command.Comment) {
	t.isExecuted = true
}

type testFeatureAllocator struct {
	isChecksEnabled bool
}

func (t testFeatureAllocator) ShouldAllocate(featureID feature.Name, featureCtx feature.FeatureContext) (bool, error) {
	return t.isChecksEnabled, nil
}

type testPrjCmdBuilder struct {
	expectedResp struct {
		prjCtxs []command.ProjectContext
		err     error
	}
}

func (t *testPrjCmdBuilder) BuildPlanCommands(ctx *command.Context, comment *command.Comment) ([]command.ProjectContext, error) {
	return t.expectedResp.prjCtxs, t.expectedResp.err
}

func (t *testPrjCmdBuilder) BuildAutoplanCommands(ctx *command.Context) ([]command.ProjectContext, error) {
	return []command.ProjectContext{}, nil
}
