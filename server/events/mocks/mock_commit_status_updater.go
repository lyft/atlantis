// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events (interfaces: CommitStatusUpdater)

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

type MockCommitStatusUpdater struct {
	fail func(message string, callerSkip ...int)
}

func NewMockCommitStatusUpdater(options ...pegomock.Option) *MockCommitStatusUpdater {
	mock := &MockCommitStatusUpdater{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockCommitStatusUpdater) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockCommitStatusUpdater) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockCommitStatusUpdater) UpdateCombined(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.CommitStatus, _param4 fmt.Stringer, _param5 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockCommitStatusUpdater().")
	}
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5}
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

func (mock *MockCommitStatusUpdater) UpdateCombinedCount(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.CommitStatus, _param4 fmt.Stringer, _param5 int, _param6 int, _param7 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockCommitStatusUpdater().")
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

func (mock *MockCommitStatusUpdater) UpdateProject(_param0 context.Context, _param1 command.ProjectContext, _param2 fmt.Stringer, _param3 models.CommitStatus, _param4 string, _param5 string) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockCommitStatusUpdater().")
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

func (mock *MockCommitStatusUpdater) VerifyWasCalledOnce() *VerifierMockCommitStatusUpdater {
	return &VerifierMockCommitStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockCommitStatusUpdater) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockCommitStatusUpdater {
	return &VerifierMockCommitStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockCommitStatusUpdater) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockCommitStatusUpdater {
	return &VerifierMockCommitStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockCommitStatusUpdater) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockCommitStatusUpdater {
	return &VerifierMockCommitStatusUpdater{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockCommitStatusUpdater struct {
	mock                   *MockCommitStatusUpdater
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockCommitStatusUpdater) UpdateCombined(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.CommitStatus, _param4 fmt.Stringer, _param5 string) *MockCommitStatusUpdater_UpdateCombined_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateCombined", params, verifier.timeout)
	return &MockCommitStatusUpdater_UpdateCombined_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockCommitStatusUpdater_UpdateCombined_OngoingVerification struct {
	mock              *MockCommitStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockCommitStatusUpdater_UpdateCombined_OngoingVerification) GetCapturedArguments() (context.Context, models.Repo, models.PullRequest, models.CommitStatus, fmt.Stringer, string) {
	_param0, _param1, _param2, _param3, _param4, _param5 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1]
}

func (c *MockCommitStatusUpdater_UpdateCombined_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []models.Repo, _param2 []models.PullRequest, _param3 []models.CommitStatus, _param4 []fmt.Stringer, _param5 []string) {
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
		_param3 = make([]models.CommitStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.CommitStatus)
		}
		_param4 = make([]fmt.Stringer, len(c.methodInvocations))
		for u, param := range params[4] {
			_param4[u] = param.(fmt.Stringer)
		}
		_param5 = make([]string, len(c.methodInvocations))
		for u, param := range params[5] {
			_param5[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockCommitStatusUpdater) UpdateCombinedCount(_param0 context.Context, _param1 models.Repo, _param2 models.PullRequest, _param3 models.CommitStatus, _param4 fmt.Stringer, _param5 int, _param6 int, _param7 string) *MockCommitStatusUpdater_UpdateCombinedCount_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5, _param6, _param7}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateCombinedCount", params, verifier.timeout)
	return &MockCommitStatusUpdater_UpdateCombinedCount_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockCommitStatusUpdater_UpdateCombinedCount_OngoingVerification struct {
	mock              *MockCommitStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockCommitStatusUpdater_UpdateCombinedCount_OngoingVerification) GetCapturedArguments() (context.Context, models.Repo, models.PullRequest, models.CommitStatus, fmt.Stringer, int, int, string) {
	_param0, _param1, _param2, _param3, _param4, _param5, _param6, _param7 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1], _param6[len(_param6)-1], _param7[len(_param7)-1]
}

func (c *MockCommitStatusUpdater_UpdateCombinedCount_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []models.Repo, _param2 []models.PullRequest, _param3 []models.CommitStatus, _param4 []fmt.Stringer, _param5 []int, _param6 []int, _param7 []string) {
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
		_param3 = make([]models.CommitStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.CommitStatus)
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

func (verifier *VerifierMockCommitStatusUpdater) UpdateProject(_param0 context.Context, _param1 command.ProjectContext, _param2 fmt.Stringer, _param3 models.CommitStatus, _param4 string, _param5 string) *MockCommitStatusUpdater_UpdateProject_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2, _param3, _param4, _param5}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UpdateProject", params, verifier.timeout)
	return &MockCommitStatusUpdater_UpdateProject_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockCommitStatusUpdater_UpdateProject_OngoingVerification struct {
	mock              *MockCommitStatusUpdater
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockCommitStatusUpdater_UpdateProject_OngoingVerification) GetCapturedArguments() (context.Context, command.ProjectContext, fmt.Stringer, models.CommitStatus, string, string) {
	_param0, _param1, _param2, _param3, _param4, _param5 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1], _param3[len(_param3)-1], _param4[len(_param4)-1], _param5[len(_param5)-1]
}

func (c *MockCommitStatusUpdater_UpdateProject_OngoingVerification) GetAllCapturedArguments() (_param0 []context.Context, _param1 []command.ProjectContext, _param2 []fmt.Stringer, _param3 []models.CommitStatus, _param4 []string, _param5 []string) {
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
		_param3 = make([]models.CommitStatus, len(c.methodInvocations))
		for u, param := range params[3] {
			_param3[u] = param.(models.CommitStatus)
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
