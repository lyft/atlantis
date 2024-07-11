// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/neptune/lyft/feature (interfaces: Allocator)

package mocks

import (
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
	feature "github.com/runatlantis/atlantis/server/neptune/lyft/feature"
)

type MockAllocator struct {
	fail func(message string, callerSkip ...int)
}

func NewMockAllocator(options ...pegomock.Option) *MockAllocator {
	mock := &MockAllocator{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockAllocator) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockAllocator) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockAllocator) ShouldAllocate(_param0 feature.Name, _param1 feature.FeatureContext) (bool, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockAllocator().")
	}
	params := []pegomock.Param{_param0, _param1}
	result := pegomock.GetGenericMockFrom(mock).Invoke("ShouldAllocate", params, []reflect.Type{reflect.TypeOf((*bool)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 bool
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(bool)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockAllocator) VerifyWasCalledOnce() *VerifierMockAllocator {
	return &VerifierMockAllocator{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockAllocator) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockAllocator {
	return &VerifierMockAllocator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockAllocator) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockAllocator {
	return &VerifierMockAllocator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockAllocator) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockAllocator {
	return &VerifierMockAllocator{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockAllocator struct {
	mock                   *MockAllocator
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockAllocator) ShouldAllocate(_param0 feature.Name, _param1 feature.FeatureContext) *MockAllocator_ShouldAllocate_OngoingVerification {
	params := []pegomock.Param{_param0, _param1}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "ShouldAllocate", params, verifier.timeout)
	return &MockAllocator_ShouldAllocate_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockAllocator_ShouldAllocate_OngoingVerification struct {
	mock              *MockAllocator
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockAllocator_ShouldAllocate_OngoingVerification) GetCapturedArguments() (feature.Name, feature.FeatureContext) {
	_param0, _param1 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1]
}

func (c *MockAllocator_ShouldAllocate_OngoingVerification) GetAllCapturedArguments() (_param0 []feature.Name, _param1 []feature.FeatureContext) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]feature.Name, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(feature.Name)
		}
		_param1 = make([]feature.FeatureContext, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(feature.FeatureContext)
		}
	}
	return
}
