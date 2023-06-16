// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/legacy/events/vcs (interfaces: GithubCredentials)

package mocks

import (
	http "net/http"
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
)

type MockGithubCredentials struct {
	fail func(message string, callerSkip ...int)
}

func NewMockGithubCredentials(options ...pegomock.Option) *MockGithubCredentials {
	mock := &MockGithubCredentials{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockGithubCredentials) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockGithubCredentials) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockGithubCredentials) Client() (*http.Client, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockGithubCredentials().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Client", params, []reflect.Type{reflect.TypeOf((**http.Client)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *http.Client
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*http.Client)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockGithubCredentials) GetToken() (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockGithubCredentials().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetToken", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockGithubCredentials) GetUser() (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockGithubCredentials().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetUser", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockGithubCredentials) VerifyWasCalledOnce() *VerifierMockGithubCredentials {
	return &VerifierMockGithubCredentials{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockGithubCredentials) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockGithubCredentials {
	return &VerifierMockGithubCredentials{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockGithubCredentials) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockGithubCredentials {
	return &VerifierMockGithubCredentials{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockGithubCredentials) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockGithubCredentials {
	return &VerifierMockGithubCredentials{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockGithubCredentials struct {
	mock                   *MockGithubCredentials
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockGithubCredentials) Client() *MockGithubCredentials_Client_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Client", params, verifier.timeout)
	return &MockGithubCredentials_Client_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockGithubCredentials_Client_OngoingVerification struct {
	mock              *MockGithubCredentials
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockGithubCredentials_Client_OngoingVerification) GetCapturedArguments() {
}

func (c *MockGithubCredentials_Client_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockGithubCredentials) GetToken() *MockGithubCredentials_GetToken_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetToken", params, verifier.timeout)
	return &MockGithubCredentials_GetToken_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockGithubCredentials_GetToken_OngoingVerification struct {
	mock              *MockGithubCredentials
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockGithubCredentials_GetToken_OngoingVerification) GetCapturedArguments() {
}

func (c *MockGithubCredentials_GetToken_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockGithubCredentials) GetUser() *MockGithubCredentials_GetUser_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetUser", params, verifier.timeout)
	return &MockGithubCredentials_GetUser_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockGithubCredentials_GetUser_OngoingVerification struct {
	mock              *MockGithubCredentials
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockGithubCredentials_GetUser_OngoingVerification) GetCapturedArguments() {
}

func (c *MockGithubCredentials_GetUser_OngoingVerification) GetAllCapturedArguments() {
}