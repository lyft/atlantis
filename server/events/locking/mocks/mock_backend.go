// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events/locking (interfaces: Backend)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
	"time"
)

type MockBackend struct {
	fail func(message string, callerSkip ...int)
}

func NewMockBackend(options ...pegomock.Option) *MockBackend {
	mock := &MockBackend{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockBackend) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockBackend) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockBackend) TryLock(lock models.ProjectLock) (bool, models.ProjectLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{lock}
	result := pegomock.GetGenericMockFrom(mock).Invoke("TryLock", params, []reflect.Type{reflect.TypeOf((*bool)(nil)).Elem(), reflect.TypeOf((*models.ProjectLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 bool
	var ret1 models.ProjectLock
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(bool)
		}
		if result[1] != nil {
			ret1 = result[1].(models.ProjectLock)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockBackend) Unlock(project models.Project, workspace string) (*models.ProjectLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{project, workspace}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Unlock", params, []reflect.Type{reflect.TypeOf((**models.ProjectLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *models.ProjectLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*models.ProjectLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) List() ([]models.ProjectLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("List", params, []reflect.Type{reflect.TypeOf((*[]models.ProjectLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 []models.ProjectLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].([]models.ProjectLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) GetLock(project models.Project, workspace string) (*models.ProjectLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{project, workspace}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetLock", params, []reflect.Type{reflect.TypeOf((**models.ProjectLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *models.ProjectLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*models.ProjectLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) UnlockByPull(repoFullName string, pullNum int) ([]models.ProjectLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{repoFullName, pullNum}
	result := pegomock.GetGenericMockFrom(mock).Invoke("UnlockByPull", params, []reflect.Type{reflect.TypeOf((*[]models.ProjectLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 []models.ProjectLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].([]models.ProjectLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) LockCommand(cmdName models.CommandName, lockTime time.Time) (models.CommandLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{cmdName, lockTime}
	result := pegomock.GetGenericMockFrom(mock).Invoke("LockCommand", params, []reflect.Type{reflect.TypeOf((*models.CommandLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 models.CommandLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(models.CommandLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) UnlockCommand(cmdName models.CommandName) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{cmdName}
	result := pegomock.GetGenericMockFrom(mock).Invoke("UnlockCommand", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockBackend) GetCommandLock(cmdName models.CommandName) (models.CommandLock, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBackend().")
	}
	params := []pegomock.Param{cmdName}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetCommandLock", params, []reflect.Type{reflect.TypeOf((*models.CommandLock)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 models.CommandLock
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(models.CommandLock)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBackend) VerifyWasCalledOnce() *VerifierMockBackend {
	return &VerifierMockBackend{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockBackend) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierMockBackend {
	return &VerifierMockBackend{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockBackend) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierMockBackend {
	return &VerifierMockBackend{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockBackend) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *VerifierMockBackend {
	return &VerifierMockBackend{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockBackend struct {
	mock                   *MockBackend
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockBackend) TryLock(lock models.ProjectLock) *MockBackend_TryLock_OngoingVerification {
	params := []pegomock.Param{lock}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "TryLock", params, verifier.timeout)
	return &MockBackend_TryLock_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_TryLock_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_TryLock_OngoingVerification) GetCapturedArguments() models.ProjectLock {
	lock := c.GetAllCapturedArguments()
	return lock[len(lock)-1]
}

func (c *MockBackend_TryLock_OngoingVerification) GetAllCapturedArguments() (_param0 []models.ProjectLock) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.ProjectLock, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.ProjectLock)
		}
	}
	return
}

func (verifier *VerifierMockBackend) Unlock(project models.Project, workspace string) *MockBackend_Unlock_OngoingVerification {
	params := []pegomock.Param{project, workspace}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Unlock", params, verifier.timeout)
	return &MockBackend_Unlock_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_Unlock_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_Unlock_OngoingVerification) GetCapturedArguments() (models.Project, string) {
	project, workspace := c.GetAllCapturedArguments()
	return project[len(project)-1], workspace[len(workspace)-1]
}

func (c *MockBackend_Unlock_OngoingVerification) GetAllCapturedArguments() (_param0 []models.Project, _param1 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.Project, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.Project)
		}
		_param1 = make([]string, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockBackend) List() *MockBackend_List_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "List", params, verifier.timeout)
	return &MockBackend_List_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_List_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_List_OngoingVerification) GetCapturedArguments() {
}

func (c *MockBackend_List_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockBackend) GetLock(project models.Project, workspace string) *MockBackend_GetLock_OngoingVerification {
	params := []pegomock.Param{project, workspace}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetLock", params, verifier.timeout)
	return &MockBackend_GetLock_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_GetLock_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_GetLock_OngoingVerification) GetCapturedArguments() (models.Project, string) {
	project, workspace := c.GetAllCapturedArguments()
	return project[len(project)-1], workspace[len(workspace)-1]
}

func (c *MockBackend_GetLock_OngoingVerification) GetAllCapturedArguments() (_param0 []models.Project, _param1 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.Project, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.Project)
		}
		_param1 = make([]string, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockBackend) UnlockByPull(repoFullName string, pullNum int) *MockBackend_UnlockByPull_OngoingVerification {
	params := []pegomock.Param{repoFullName, pullNum}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UnlockByPull", params, verifier.timeout)
	return &MockBackend_UnlockByPull_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_UnlockByPull_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_UnlockByPull_OngoingVerification) GetCapturedArguments() (string, int) {
	repoFullName, pullNum := c.GetAllCapturedArguments()
	return repoFullName[len(repoFullName)-1], pullNum[len(pullNum)-1]
}

func (c *MockBackend_UnlockByPull_OngoingVerification) GetAllCapturedArguments() (_param0 []string, _param1 []int) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
		_param1 = make([]int, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(int)
		}
	}
	return
}

func (verifier *VerifierMockBackend) LockCommand(cmdName models.CommandName, lockTime time.Time) *MockBackend_LockCommand_OngoingVerification {
	params := []pegomock.Param{cmdName, lockTime}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "LockCommand", params, verifier.timeout)
	return &MockBackend_LockCommand_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_LockCommand_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_LockCommand_OngoingVerification) GetCapturedArguments() (models.CommandName, time.Time) {
	cmdName, lockTime := c.GetAllCapturedArguments()
	return cmdName[len(cmdName)-1], lockTime[len(lockTime)-1]
}

func (c *MockBackend_LockCommand_OngoingVerification) GetAllCapturedArguments() (_param0 []models.CommandName, _param1 []time.Time) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.CommandName, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.CommandName)
		}
		_param1 = make([]time.Time, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(time.Time)
		}
	}
	return
}

func (verifier *VerifierMockBackend) UnlockCommand(cmdName models.CommandName) *MockBackend_UnlockCommand_OngoingVerification {
	params := []pegomock.Param{cmdName}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "UnlockCommand", params, verifier.timeout)
	return &MockBackend_UnlockCommand_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_UnlockCommand_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_UnlockCommand_OngoingVerification) GetCapturedArguments() models.CommandName {
	cmdName := c.GetAllCapturedArguments()
	return cmdName[len(cmdName)-1]
}

func (c *MockBackend_UnlockCommand_OngoingVerification) GetAllCapturedArguments() (_param0 []models.CommandName) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.CommandName, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.CommandName)
		}
	}
	return
}

func (verifier *VerifierMockBackend) GetCommandLock(cmdName models.CommandName) *MockBackend_GetCommandLock_OngoingVerification {
	params := []pegomock.Param{cmdName}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetCommandLock", params, verifier.timeout)
	return &MockBackend_GetCommandLock_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockBackend_GetCommandLock_OngoingVerification struct {
	mock              *MockBackend
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockBackend_GetCommandLock_OngoingVerification) GetCapturedArguments() models.CommandName {
	cmdName := c.GetAllCapturedArguments()
	return cmdName[len(cmdName)-1]
}

func (c *MockBackend_GetCommandLock_OngoingVerification) GetAllCapturedArguments() (_param0 []models.CommandName) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.CommandName, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.CommandName)
		}
	}
	return
}
