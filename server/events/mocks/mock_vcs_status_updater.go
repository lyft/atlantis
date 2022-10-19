// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events (interfaces: VCSStatusUpdater)

package mocks

import (
	context "context"
	fmt "fmt"
	pegomock "github.com/petergtz/pegomock"
	command "github.com/runatlantis/atlantis/server/events/command"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
	"time"
)

type MockVCSStatusUpdater struct {
	fail func(message string, callerSkip ...int)
}

func NewMockVCSStatusUpdater(options ...pegomock.Option) *MockVCSStatusUpdater {
	mock := &MockVCSStatusUpdater{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockVCSStatusUpdater) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockVCSStatusUpdater) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockVCSStatusUpdater) UpdateCombined(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.VcsStatus, _param4 fmt.Stringer, _param5 string, _param6 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockVCSStatusUpdater().")
	}
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5, _param6}
	result := pegomock.GetGenericMockFrom(mock).Invoke("UpdateCombined", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockVCSStatusUpdater) UpdateCombinedCount(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.VcsStatus, _param4 fmt.Stringer, _param5 int, _param6 int, _param7 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockVCSStatusUpdater().")
	}
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5, _param6, _param7}
	result := pegomock.GetGenericMockFrom(mock).Invoke("UpdateCombinedCount", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockVCSStatusUpdater) UpdateProject(_param0 context.Context, _param1 command.ProjectContext, _param2 fmt.Stringer, _param3 models.VcsStatus, _param4 string, _param5 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockVCSStatusUpdater().")
	}
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5}
	result := pegomock.GetGenericMockFrom(mock).Invoke("UpdateProject", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockVCSStatusUpdater) VerifyWasCalledOnce() *VerifierMockVCSStatusUpdater {
	return &VerifierMockVCSStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockVCSStatusUpdater) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockVCSStatusUpdater {
	return &VerifierMockVCSStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockVCSStatusUpdater) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockVCSStatusUpdater {
	return &VerifierMockVCSStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockVCSStatusUpdater) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockVCSStatusUpdater {
	return &VerifierMockVCSStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockVCSStatusUpdater struct {
	mock                   *MockVCSStatusUpdater
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockVCSStatusUpdater) UpdateCombined(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.VcsStatus, _param4 fmt.Stringer, _param5 string, _param6 string) *MockVCSStatusUpdater_UpdateCombined_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5, _param6}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateCombined", params, verifier.timeout)
	return &MockVCSStatusUpdater_UpdateCombined_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockVCSStatusUpdater_UpdateCombined_OngoingVerification struct {
	mock              *MockVCSStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockVCSStatusUpdater_UpdateCombined_OngoingVerification) GetCapturedArguments() (context.Context, models.Repo, models.PullRequest, models.VcsStatus, fmt.Stringer, string, string) {
	_param0, _param1, _param2, _param3, _param4, _param5, _param6 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1], _param6[len(_param6)-1]
}

func (c *MockVCSStatusUpdater_UpdateCombined_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []models.Repo, _param2 []models.PullRequest, _param3 []models.VcsStatus, _param4 []fmt.Stringer, _param5 []string, _param6 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]context.Context, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(context.Context)
		}
		_param1 = make([]models.Repo, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.Repo)
		}
		_param2 = make([]models.PullRequest, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(models.PullRequest)
		}
		_param3 = make([]models.VcsStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.VcsStatus)
		}
		_param4 = make([]fmt.Stringer, len(c.methodInvocations))
		for u, param := range params[4] {
			_param4[u] = param.(fmt.Stringer)
		}
		_param5 = make([]string, len(c.methodInvocations))
		for u, param := range params[5] {
			_param5[u] = param.(string)
		}
		_param6 = make([]string, len(c.methodInvocations))
		for u, param := range params[6] {
			_param6[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockVCSStatusUpdater) UpdateCombinedCount(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.VcsStatus, _param4 fmt.Stringer, _param5 int, _param6 int, _param7 string) *MockVCSStatusUpdater_UpdateCombinedCount_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5, _param6, _param7}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateCombinedCount", params, verifier.timeout)
	return &MockVCSStatusUpdater_UpdateCombinedCount_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockVCSStatusUpdater_UpdateCombinedCount_OngoingVerification struct {
	mock              *MockVCSStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockVCSStatusUpdater_UpdateCombinedCount_OngoingVerification) GetCapturedArguments() (context.Context, models.Repo, models.PullRequest, models.VcsStatus, fmt.Stringer, int, int, string) {
	_param0, _param1, _param2, _param3, _param4, _param5, _param6, _param7 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1], _param6[len(_param6)-1], _param7[len(_param7)-1]
}

func (c *MockVCSStatusUpdater_UpdateCombinedCount_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []models.Repo, _param2 []models.PullRequest, _param3 []models.VcsStatus, _param4 []fmt.Stringer, _param5 []int, _param6 []int, _param7 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]context.Context, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(context.Context)
		}
		_param1 = make([]models.Repo, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.Repo)
		}
		_param2 = make([]models.PullRequest, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(models.PullRequest)
		}
		_param3 = make([]models.VcsStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.VcsStatus)
		}
		_param4 = make([]fmt.Stringer, len(c.methodInvocations))
		for u, param := range params[4] {
			_param4[u] = param.(fmt.Stringer)
		}
		_param5 = make([]int, len(c.methodInvocations))
		for u, param := range params[5] {
			_param5[u] = param.(int)
		}
		_param6 = make([]int, len(c.methodInvocations))
		for u, param := range params[6] {
			_param6[u] = param.(int)
		}
		_param7 = make([]string, len(c.methodInvocations))
		for u, param := range params[7] {
			_param7[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockVCSStatusUpdater) UpdateProject(_param0 context.Context, _param1 command.ProjectContext, _param2 fmt.Stringer, _param3 models.VcsStatus, _param4 string, _param5 string) *MockVCSStatusUpdater_UpdateProject_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateProject", params, verifier.timeout)
	return &MockVCSStatusUpdater_UpdateProject_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockVCSStatusUpdater_UpdateProject_OngoingVerification struct {
	mock              *MockVCSStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockVCSStatusUpdater_UpdateProject_OngoingVerification) GetCapturedArguments() (context.Context, command.ProjectContext, fmt.Stringer, models.VcsStatus, string, string) {
	_param0, _param1, _param2, _param3, _param4, _param5 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1]
}

func (c *MockVCSStatusUpdater_UpdateProject_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []command.ProjectContext, _param2 []fmt.Stringer, _param3 []models.VcsStatus, _param4 []string, _param5 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]context.Context, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(context.Context)
		}
		_param1 = make([]command.ProjectContext, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(command.ProjectContext)
		}
		_param2 = make([]fmt.Stringer, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(fmt.Stringer)
		}
		_param3 = make([]models.VcsStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.VcsStatus)
		}
		_param4 = make([]string, len(c.methodInvocations))
		for u, param := range params[4] {
			_param4[u] = param.(string)
		}
		_param5 = make([]string, len(c.methodInvocations))
		for u, param := range params[5] {
			_param5[u] = param.(string)
		}
	}
	return
}
