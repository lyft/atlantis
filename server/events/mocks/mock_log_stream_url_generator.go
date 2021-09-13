// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events (interfaces: JobsUrlGenerator)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
	"time"
)

type MockJobsUrlGenerator struct {
	fail func(message string, callerSkip ...int)
}

func NewMockJobsUrlGenerator(options ...pegomock.Option) *MockJobsUrlGenerator {
	mock := &MockJobsUrlGenerator{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockJobsUrlGenerator) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockJobsUrlGenerator) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockJobsUrlGenerator) GenerateProjectJobsUrl(pull models.PullRequest, p models.ProjectCommandContext) string {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockJobsUrlGenerator().")
	}
	params := []pegomock.Param{pull, p}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GenerateProjectJobsUrl", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem()})
	var ret0 string
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
	}
	return ret0
}

func (mock *MockJobsUrlGenerator) PullRequestJobsUrl(pull models.PullRequest) string {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockJobsUrlGenerator().")
	}
	params := []pegomock.Param{pull}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GenerateProjectJobsUrl", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem()})
	var ret0 string
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
	}
	return ret0
}

func (mock *MockJobsUrlGenerator) VerifyWasCalledOnce() *VerifierMockJobsUrlGenerator {
	return &VerifierMockJobsUrlGenerator{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockJobsUrlGenerator) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockJobsUrlGenerator {
	return &VerifierMockJobsUrlGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockJobsUrlGenerator) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockJobsUrlGenerator {
	return &VerifierMockJobsUrlGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockJobsUrlGenerator) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockJobsUrlGenerator {
	return &VerifierMockJobsUrlGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockJobsUrlGenerator struct {
	mock                   *MockJobsUrlGenerator
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockJobsUrlGenerator) GenerateProjectJobsUrl(pull models.PullRequest, p models.ProjectCommandContext) *MockJobsUrlGenerator_GenerateProjectJobsUrl_OngoingVerification {
	params := []pegomock.Param{pull, p}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GenerateProjectJobsUrl", params, verifier.timeout)
	return &MockJobsUrlGenerator_GenerateProjectJobsUrl_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockJobsUrlGenerator_GenerateProjectJobsUrl_OngoingVerification struct {
	mock              *MockJobsUrlGenerator
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockJobsUrlGenerator_GenerateProjectJobsUrl_OngoingVerification) GetCapturedArguments() (models.PullRequest, models.ProjectCommandContext) {
	pull, p := c.GetAllCapturedArguments()
	return pull[len(pull)-1], p[len(p)-1]
}

func (c *MockJobsUrlGenerator_GenerateProjectJobsUrl_OngoingVerification) GetAllCapturedArguments() (_param0 []models.PullRequest, _param1 []models.ProjectCommandContext) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.PullRequest, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.PullRequest)
		}
		_param1 = make([]models.ProjectCommandContext, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(models.ProjectCommandContext)
		}
	}
	return
}
