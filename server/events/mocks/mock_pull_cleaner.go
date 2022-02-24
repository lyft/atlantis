// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events (interfaces: PullCleaner)

package mocks

import (
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
)

type MockPullCleaner struct {
	fail func(message string, callerSkip ...int)
}

func NewMockPullCleaner(options ...pegomock.Option) *MockPullCleaner {
	mock := &MockPullCleaner{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockPullCleaner) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockPullCleaner) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockPullCleaner) CleanUpPull(_param0 models.Repo, _param1 models.PullRequest) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockPullCleaner().")
	}
	params := []pegomock.Param{_param0, _param1}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CleanUpPull", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockPullCleaner) VerifyWasCalledOnce() *VerifierMockPullCleaner {
	return &VerifierMockPullCleaner{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockPullCleaner) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockPullCleaner {
	return &VerifierMockPullCleaner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockPullCleaner) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockPullCleaner {
	return &VerifierMockPullCleaner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockPullCleaner) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockPullCleaner {
	return &VerifierMockPullCleaner{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockPullCleaner struct {
	mock                   *MockPullCleaner
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockPullCleaner) CleanUpPull(_param0 models.Repo, _param1 models.PullRequest) *MockPullCleaner_CleanUpPull_OngoingVerification {
	params := []pegomock.Param{_param0, _param1}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CleanUpPull", params, verifier.timeout)
	return &MockPullCleaner_CleanUpPull_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockPullCleaner_CleanUpPull_OngoingVerification struct {
	mock              *MockPullCleaner
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockPullCleaner_CleanUpPull_OngoingVerification) GetCapturedArguments() (models.Repo, models.PullRequest) {
	_param0, _param1 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1]
}

func (c *MockPullCleaner_CleanUpPull_OngoingVerification) GetAllCapturedArguments() (_param0 []models.Repo, _param1 []models.PullRequest) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.Repo, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.Repo)
		}
		_param1 = make([]models.PullRequest, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.PullRequest)
		}
	}
	return
}
