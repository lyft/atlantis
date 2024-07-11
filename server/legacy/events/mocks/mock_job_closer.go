// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/legacy/events (interfaces: JobCloser)

package mocks

import (
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/models"
)

type MockJobCloser struct {
	fail func(message string, callerSkip ...int)
}

func NewMockJobCloser(options ...pegomock.Option) *MockJobCloser {
	mock := &MockJobCloser{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockJobCloser) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockJobCloser) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockJobCloser) CloseJob(_param0 string, _param1 models.Repo) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockJobCloser().")
	}
	params := []pegomock.Param{_param0, _param1}
	pegomock.GetGenericMockFrom(mock).Invoke("CloseJob", params, []reflect.Type{})
}

func (mock *MockJobCloser) VerifyWasCalledOnce() *VerifierMockJobCloser {
	return &VerifierMockJobCloser{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockJobCloser) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockJobCloser {
	return &VerifierMockJobCloser{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockJobCloser) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockJobCloser {
	return &VerifierMockJobCloser{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockJobCloser) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockJobCloser {
	return &VerifierMockJobCloser{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockJobCloser struct {
	mock                   *MockJobCloser
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockJobCloser) CloseJob(_param0 string, _param1 models.Repo) *MockJobCloser_CloseJob_OngoingVerification {
	params := []pegomock.Param{_param0, _param1}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CloseJob", params, verifier.timeout)
	return &MockJobCloser_CloseJob_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockJobCloser_CloseJob_OngoingVerification struct {
	mock              *MockJobCloser
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockJobCloser_CloseJob_OngoingVerification) GetCapturedArguments() (string, models.Repo) {
	_param0, _param1 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1]
}

func (c *MockJobCloser_CloseJob_OngoingVerification) GetAllCapturedArguments() (_param0 []string, _param1 []models.Repo) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
		_param1 = make([]models.Repo, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.Repo)
		}
	}
	return
}
