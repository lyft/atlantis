// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/handlers (interfaces: WebsocketConnectionWrapper)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	"reflect"
	"time"
)

type MockWebsocketConnectionWrapper struct {
	fail func(message string, callerSkip ...int)
}

func NewMockWebsocketConnectionWrapper(options ...pegomock.Option) *MockWebsocketConnectionWrapper {
	mock := &MockWebsocketConnectionWrapper{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockWebsocketConnectionWrapper) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockWebsocketConnectionWrapper) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockWebsocketConnectionWrapper) ReadMessage() (int, []byte, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockWebsocketConnectionWrapper().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("ReadMessage", params, []reflect.Type{reflect.TypeOf((*int)(nil)).Elem(), reflect.TypeOf((*[]byte)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 int
	var ret1 []byte
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(int)
		}
		if result[1] != nil {
			ret1 = result[1].([]byte)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockWebsocketConnectionWrapper) SetCloseHandler(_param0 func(int, string) error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockWebsocketConnectionWrapper().")
	}
	params := []pegomock.Param{_param0}
	pegomock.GetGenericMockFrom(mock).Invoke("SetCloseHandler", params, []reflect.Type{})
}

func (mock *MockWebsocketConnectionWrapper) WriteMessage(_param0 int, _param1 []byte) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockWebsocketConnectionWrapper().")
	}
	params := []pegomock.Param{_param0, _param1}
	result := pegomock.GetGenericMockFrom(mock).Invoke("WriteMessage", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockWebsocketConnectionWrapper) VerifyWasCalledOnce() *VerifierMockWebsocketConnectionWrapper {
	return &VerifierMockWebsocketConnectionWrapper{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockWebsocketConnectionWrapper) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockWebsocketConnectionWrapper {
	return &VerifierMockWebsocketConnectionWrapper{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockWebsocketConnectionWrapper) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockWebsocketConnectionWrapper {
	return &VerifierMockWebsocketConnectionWrapper{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockWebsocketConnectionWrapper) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockWebsocketConnectionWrapper {
	return &VerifierMockWebsocketConnectionWrapper{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockWebsocketConnectionWrapper struct {
	mock                   *MockWebsocketConnectionWrapper
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockWebsocketConnectionWrapper) ReadMessage() *MockWebsocketConnectionWrapper_ReadMessage_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "ReadMessage", params, verifier.timeout)
	return &MockWebsocketConnectionWrapper_ReadMessage_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockWebsocketConnectionWrapper_ReadMessage_OngoingVerification struct {
	mock              *MockWebsocketConnectionWrapper
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockWebsocketConnectionWrapper_ReadMessage_OngoingVerification) GetCapturedArguments() {
}

func (c *MockWebsocketConnectionWrapper_ReadMessage_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockWebsocketConnectionWrapper) SetCloseHandler(_param0 func(int, string) error) *MockWebsocketConnectionWrapper_SetCloseHandler_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "SetCloseHandler", params, verifier.timeout)
	return &MockWebsocketConnectionWrapper_SetCloseHandler_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockWebsocketConnectionWrapper_SetCloseHandler_OngoingVerification struct {
	mock              *MockWebsocketConnectionWrapper
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockWebsocketConnectionWrapper_SetCloseHandler_OngoingVerification) GetCapturedArguments() func(int, string) error {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *MockWebsocketConnectionWrapper_SetCloseHandler_OngoingVerification) GetAllCapturedArguments() (_param0 []func(int, string) error) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]func(int, string) error, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(func(int, string) error)
		}
	}
	return
}

func (verifier *VerifierMockWebsocketConnectionWrapper) WriteMessage(_param0 int, _param1 []byte) *MockWebsocketConnectionWrapper_WriteMessage_OngoingVerification {
	params := []pegomock.Param{_param0, _param1}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "WriteMessage", params, verifier.timeout)
	return &MockWebsocketConnectionWrapper_WriteMessage_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockWebsocketConnectionWrapper_WriteMessage_OngoingVerification struct {
	mock              *MockWebsocketConnectionWrapper
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockWebsocketConnectionWrapper_WriteMessage_OngoingVerification) GetCapturedArguments() (int, []byte) {
	_param0, _param1 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1]
}

func (c *MockWebsocketConnectionWrapper_WriteMessage_OngoingVerification) GetAllCapturedArguments() (_param0 []int, _param1 [][]byte) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]int, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(int)
		}
		_param1 = make([][]byte, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.([]byte)
		}
	}
	return
}
