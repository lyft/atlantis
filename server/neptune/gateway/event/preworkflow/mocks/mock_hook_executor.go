// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/neptune/gateway/event/preworkflow (interfaces: HookExecutor)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	valid "github.com/runatlantis/atlantis/server/core/config/valid"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
	"time"
)

type MockHookExecutor struct {
	fail func(message string, callerSkip ...int)
}

func NewMockHookExecutor(options ...pegomock.Option) *MockHookExecutor {
	mock := &MockHookExecutor{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockHookExecutor) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockHookExecutor) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockHookExecutor) Execute(hook *valid.PreWorkflowHook, repo models.Repo, path string) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockHookExecutor().")
	}
	params := []pegomock.Param{hook, repo, path}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Execute", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockHookExecutor) VerifyWasCalledOnce() *VerifierMockHookExecutor {
	return &VerifierMockHookExecutor{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockHookExecutor) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockHookExecutor {
	return &VerifierMockHookExecutor{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockHookExecutor) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockHookExecutor {
	return &VerifierMockHookExecutor{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockHookExecutor) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockHookExecutor {
	return &VerifierMockHookExecutor{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockHookExecutor struct {
	mock                   *MockHookExecutor
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockHookExecutor) Execute(hook *valid.PreWorkflowHook, repo models.Repo, path string) *MockHookExecutor_Execute_OngoingVerification {
	params := []pegomock.Param{hook, repo, path}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Execute", params, verifier.timeout)
	return &MockHookExecutor_Execute_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockHookExecutor_Execute_OngoingVerification struct {
	mock              *MockHookExecutor
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockHookExecutor_Execute_OngoingVerification) GetCapturedArguments() (*valid.PreWorkflowHook, models.Repo, string) {
	hook, repo, path := c.GetAllCapturedArguments()
	return hook[len(hook)-1], repo[len(repo)-1], path[len(path)-1]
}

func (c *MockHookExecutor_Execute_OngoingVerification) GetAllCapturedArguments() (_param0 []*valid.PreWorkflowHook, _param1 []models.Repo, _param2 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]*valid.PreWorkflowHook, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(*valid.PreWorkflowHook)
		}
		_param1 = make([]models.Repo, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.Repo)
		}
		_param2 = make([]string, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(string)
		}
	}
	return
}