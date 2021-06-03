// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events/runtime (interfaces: VersionedExecutorWorkflow)

package mocks

import (
	go_version "github.com/hashicorp/go-version"
	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
	logging "github.com/runatlantis/atlantis/server/logging"
	"reflect"
	"time"
)

type MockVersionedExecutorWorkflow struct {
	fail func(message string, callerSkip ...int)
}

func NewMockVersionedExecutorWorkflow(options ...pegomock.Option) *MockVersionedExecutorWorkflow {
	mock := &MockVersionedExecutorWorkflow{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockVersionedExecutorWorkflow) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockVersionedExecutorWorkflow) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockVersionedExecutorWorkflow) EnsureExecutorVersion(log logging.SimpleLogging, v *go_version.Version) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockVersionedExecutorWorkflow().")
	}
	params := []pegomock.Param{log, v}
	result := pegomock.GetGenericMockFrom(mock).Invoke("EnsureExecutorVersion", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 string
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockVersionedExecutorWorkflow) Run(ctx models.ProjectCommandContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockVersionedExecutorWorkflow().")
	}
	params := []pegomock.Param{ctx, executablePath, envs, workdir, extraArgs}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Run", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 string
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockVersionedExecutorWorkflow) VerifyWasCalledOnce() *VerifierMockVersionedExecutorWorkflow {
	return &VerifierMockVersionedExecutorWorkflow{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockVersionedExecutorWorkflow) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierMockVersionedExecutorWorkflow {
	return &VerifierMockVersionedExecutorWorkflow{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockVersionedExecutorWorkflow) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierMockVersionedExecutorWorkflow {
	return &VerifierMockVersionedExecutorWorkflow{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockVersionedExecutorWorkflow) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *VerifierMockVersionedExecutorWorkflow {
	return &VerifierMockVersionedExecutorWorkflow{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockVersionedExecutorWorkflow struct {
	mock                   *MockVersionedExecutorWorkflow
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockVersionedExecutorWorkflow) EnsureExecutorVersion(log logging.SimpleLogging, v *go_version.Version) *MockVersionedExecutorWorkflow_EnsureExecutorVersion_OngoingVerification {
	params := []pegomock.Param{log, v}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "EnsureExecutorVersion", params, verifier.timeout)
	return &MockVersionedExecutorWorkflow_EnsureExecutorVersion_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockVersionedExecutorWorkflow_EnsureExecutorVersion_OngoingVerification struct {
	mock              *MockVersionedExecutorWorkflow
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockVersionedExecutorWorkflow_EnsureExecutorVersion_OngoingVerification) GetCapturedArguments() (logging.SimpleLogging, *go_version.Version) {
	log, v := c.GetAllCapturedArguments()
	return log[len(log)-1], v[len(v)-1]
}

func (c *MockVersionedExecutorWorkflow_EnsureExecutorVersion_OngoingVerification) GetAllCapturedArguments() (_param0 []logging.SimpleLogging, _param1 []*go_version.Version) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]logging.SimpleLogging, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(logging.SimpleLogging)
		}
		_param1 = make([]*go_version.Version, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(*go_version.Version)
		}
	}
	return
}

func (verifier *VerifierMockVersionedExecutorWorkflow) Run(ctx models.ProjectCommandContext, executablePath string, envs map[string]string, workdir string, extraArgs []string) *MockVersionedExecutorWorkflow_Run_OngoingVerification {
	params := []pegomock.Param{ctx, executablePath, envs, workdir, extraArgs}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Run", params, verifier.timeout)
	return &MockVersionedExecutorWorkflow_Run_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockVersionedExecutorWorkflow_Run_OngoingVerification struct {
	mock              *MockVersionedExecutorWorkflow
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockVersionedExecutorWorkflow_Run_OngoingVerification) GetCapturedArguments() (models.ProjectCommandContext, string, map[string]string, string, []string) {
	ctx, executablePath, envs, workdir, extraArgs := c.GetAllCapturedArguments()
	return ctx[len(ctx)-1], executablePath[len(executablePath)-1], envs[len(envs)-1], workdir[len(workdir)-1], extraArgs[len(extraArgs)-1]
}

func (c *MockVersionedExecutorWorkflow_Run_OngoingVerification) GetAllCapturedArguments() (_param0 []models.ProjectCommandContext, _param1 []string, _param2 []map[string]string, _param3 []string, _param4 [][]string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.ProjectCommandContext, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.ProjectCommandContext)
		}
		_param1 = make([]string, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(string)
		}
		_param2 = make([]map[string]string, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(map[string]string)
		}
		_param3 = make([]string, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(string)
		}
		_param4 = make([][]string, len(c.methodInvocations))
		for u, param := range params[4] {
			_param4[u] = param.([]string)
		}
	}
	return
}
