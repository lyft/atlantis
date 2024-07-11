// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/legacy/events (interfaces: PreWorkflowHooksCommandRunner)

package mocks

import (
	context "context"
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
	command "github.com/runatlantis/atlantis/server/legacy/events/command"
)

type MockPreWorkflowHooksCommandRunner struct {
	fail func(message string, callerSkip ...int)
}

func NewMockPreWorkflowHooksCommandRunner(options ...pegomock.Option) *MockPreWorkflowHooksCommandRunner {
	mock := &MockPreWorkflowHooksCommandRunner{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockPreWorkflowHooksCommandRunner) SetFailHandler(fh pegomock.FailHandler) {
	mock.fail = fh
}
func (mock *MockPreWorkflowHooksCommandRunner) FailHandler() pegomock.FailHandler { return mock.fail }

func (mock *MockPreWorkflowHooksCommandRunner) RunPreHooks(ctx context.Context, cmdCtx *command.Context) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockPreWorkflowHooksCommandRunner().")
	}
	params := []pegomock.Param{ctx, cmdCtx}
	result := pegomock.GetGenericMockFrom(mock).Invoke("RunPreHooks", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockPreWorkflowHooksCommandRunner) VerifyWasCalledOnce() *VerifierMockPreWorkflowHooksCommandRunner {
	return &VerifierMockPreWorkflowHooksCommandRunner{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockPreWorkflowHooksCommandRunner) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierMockPreWorkflowHooksCommandRunner {
	return &VerifierMockPreWorkflowHooksCommandRunner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockPreWorkflowHooksCommandRunner) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierMockPreWorkflowHooksCommandRunner {
	return &VerifierMockPreWorkflowHooksCommandRunner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockPreWorkflowHooksCommandRunner) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *VerifierMockPreWorkflowHooksCommandRunner {
	return &VerifierMockPreWorkflowHooksCommandRunner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockPreWorkflowHooksCommandRunner struct {
	mock                   *MockPreWorkflowHooksCommandRunner
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockPreWorkflowHooksCommandRunner) RunPreHooks(ctx context.Context, cmdCtx *command.Context) *MockPreWorkflowHooksCommandRunner_RunPreHooks_OngoingVerification {
	params := []pegomock.Param{ctx, cmdCtx}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "RunPreHooks", params, verifier.timeout)
	return &MockPreWorkflowHooksCommandRunner_RunPreHooks_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockPreWorkflowHooksCommandRunner_RunPreHooks_OngoingVerification struct {
	mock              *MockPreWorkflowHooksCommandRunner
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockPreWorkflowHooksCommandRunner_RunPreHooks_OngoingVerification) GetCapturedArguments() (context.Context, *command.Context) {
	ctx, cmdCtx := c.GetAllCapturedArguments()
	return ctx[len(ctx)-1], cmdCtx[len(cmdCtx)-1]
}

func (c *MockPreWorkflowHooksCommandRunner_RunPreHooks_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []*command.Context) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]context.Context, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(context.Context)
		}
		_param1 = make([]*command.Context, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(*command.Context)
		}
	}
	return
}
