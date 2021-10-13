// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/handlers (interfaces: ProjectJobURLGenerator)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
	"time"
)

type MockProjectJobURLGenerator struct {
	fail func(message string, callerSkip ...int)
}

func NewMockProjectJobURLGenerator(options ...pegomock.Option) *MockProjectJobURLGenerator {
	mock := &MockProjectJobURLGenerator{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockProjectJobURLGenerator) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockProjectJobURLGenerator) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockProjectJobURLGenerator) GenerateProjectJobURL(p models.ProjectCommandContext) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockProjectJobURLGenerator().")
	}
	params := []pegomock.Param{p}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GenerateProjectJobURL", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
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

func (mock *MockProjectJobURLGenerator) VerifyWasCalledOnce() *VerifierMockProjectJobURLGenerator {
	return &VerifierMockProjectJobURLGenerator{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockProjectJobURLGenerator) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockProjectJobURLGenerator {
	return &VerifierMockProjectJobURLGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockProjectJobURLGenerator) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockProjectJobURLGenerator {
	return &VerifierMockProjectJobURLGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockProjectJobURLGenerator) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockProjectJobURLGenerator {
	return &VerifierMockProjectJobURLGenerator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockProjectJobURLGenerator struct {
	mock                   *MockProjectJobURLGenerator
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockProjectJobURLGenerator) GenerateProjectJobURL(p models.ProjectCommandContext) *MockProjectJobURLGenerator_GenerateProjectJobURL_OngoingVerification {
	params := []pegomock.Param{p}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GenerateProjectJobURL", params, verifier.timeout)
	return &MockProjectJobURLGenerator_GenerateProjectJobURL_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockProjectJobURLGenerator_GenerateProjectJobURL_OngoingVerification struct {
	mock              *MockProjectJobURLGenerator
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockProjectJobURLGenerator_GenerateProjectJobURL_OngoingVerification) GetCapturedArguments() models.ProjectCommandContext {
	p := c.GetAllCapturedArguments()
	return p[len(p)-1]
}

func (c *MockProjectJobURLGenerator_GenerateProjectJobURL_OngoingVerification) GetAllCapturedArguments() (_param0 []models.ProjectCommandContext) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]models.ProjectCommandContext, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(models.ProjectCommandContext)
		}
	}
	return
}
