// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/legacy/events (interfaces: WebhooksSender)

package mocks

import (
	"reflect"
	"time"

	pegomock "github.com/petergtz/pegomock"
	webhooks "github.com/runatlantis/atlantis/server/legacy/events/webhooks"
	logging "github.com/runatlantis/atlantis/server/logging"
)

type MockWebhooksSender struct {
	fail func(message string, callerSkip ...int)
}

func NewMockWebhooksSender(options ...pegomock.Option) *MockWebhooksSender {
	mock := &MockWebhooksSender{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockWebhooksSender) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockWebhooksSender) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockWebhooksSender) Send(log logging.Logger, res webhooks.ApplyResult) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockWebhooksSender().")
	}
	params := []pegomock.Param{log, res}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Send", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockWebhooksSender) VerifyWasCalledOnce() *VerifierMockWebhooksSender {
	return &VerifierMockWebhooksSender{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockWebhooksSender) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockWebhooksSender {
	return &VerifierMockWebhooksSender{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockWebhooksSender) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockWebhooksSender {
	return &VerifierMockWebhooksSender{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockWebhooksSender) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockWebhooksSender {
	return &VerifierMockWebhooksSender{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockWebhooksSender struct {
	mock                   *MockWebhooksSender
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockWebhooksSender) Send(log logging.Logger, res webhooks.ApplyResult) *MockWebhooksSender_Send_OngoingVerification {
	params := []pegomock.Param{log, res}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Send", params, verifier.timeout)
	return &MockWebhooksSender_Send_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockWebhooksSender_Send_OngoingVerification struct {
	mock              *MockWebhooksSender
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockWebhooksSender_Send_OngoingVerification) GetCapturedArguments() (logging.Logger, webhooks.ApplyResult) {
	log, res := c.GetAllCapturedArguments()
	return log[len(log)-1], res[len(res)-1]
}

func (c *MockWebhooksSender_Send_OngoingVerification) GetAllCapturedArguments() (_param0 []logging.Logger, _param1 []webhooks.ApplyResult) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]logging.Logger, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(logging.Logger)
		}
		_param1 = make([]webhooks.ApplyResult, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(webhooks.ApplyResult)
		}
	}
	return
}
