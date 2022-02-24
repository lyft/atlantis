// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events/webhooks (interfaces: UnderlyingSlackClient)

package mocks

import (
	"reflect"
	"time"

	slack "github.com/nlopes/slack"
	pegomock "github.com/petergtz/pegomock"
)

type MockUnderlyingSlackClient struct {
	fail func(message string, callerSkip ...int)
}

func NewMockUnderlyingSlackClient(options ...pegomock.Option) *MockUnderlyingSlackClient {
	mock := &MockUnderlyingSlackClient{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockUnderlyingSlackClient) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockUnderlyingSlackClient) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockUnderlyingSlackClient) AuthTest() (*slack.AuthTestResponse, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockUnderlyingSlackClient().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("AuthTest", params, []reflect.Type{reflect.TypeOf((**slack.AuthTestResponse)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *slack.AuthTestResponse
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*slack.AuthTestResponse)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockUnderlyingSlackClient) GetConversations(conversationParams *slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockUnderlyingSlackClient().")
	}
	params := []pegomock.Param{conversationParams}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetConversations", params, []reflect.Type{reflect.TypeOf((*[]slack.Channel)(nil)).Elem(), reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 []slack.Channel
	var ret1 string
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].([]slack.Channel)
		}
		if result[1] != nil {
			ret1 = result[1].(string)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockUnderlyingSlackClient) PostMessage(channel string, text string, parameters slack.PostMessageParameters) (string, string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockUnderlyingSlackClient().")
	}
	params := []pegomock.Param{channel, text, parameters}
	result := pegomock.GetGenericMockFrom(mock).Invoke("PostMessage", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 string
	var ret1 string
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
		if result[1] != nil {
			ret1 = result[1].(string)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockUnderlyingSlackClient) VerifyWasCalledOnce() *VerifierMockUnderlyingSlackClient {
	return &VerifierMockUnderlyingSlackClient{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockUnderlyingSlackClient) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockUnderlyingSlackClient {
	return &VerifierMockUnderlyingSlackClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockUnderlyingSlackClient) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockUnderlyingSlackClient {
	return &VerifierMockUnderlyingSlackClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockUnderlyingSlackClient) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockUnderlyingSlackClient {
	return &VerifierMockUnderlyingSlackClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockUnderlyingSlackClient struct {
	mock                   *MockUnderlyingSlackClient
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockUnderlyingSlackClient) AuthTest() *MockUnderlyingSlackClient_AuthTest_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "AuthTest", params, verifier.timeout)
	return &MockUnderlyingSlackClient_AuthTest_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockUnderlyingSlackClient_AuthTest_OngoingVerification struct {
	mock              *MockUnderlyingSlackClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockUnderlyingSlackClient_AuthTest_OngoingVerification) GetCapturedArguments() {
}

func (c *MockUnderlyingSlackClient_AuthTest_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockUnderlyingSlackClient) GetConversations(conversationParams *slack.GetConversationsParameters) *MockUnderlyingSlackClient_GetConversations_OngoingVerification {
	params := []pegomock.Param{conversationParams}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetConversations", params, verifier.timeout)
	return &MockUnderlyingSlackClient_GetConversations_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockUnderlyingSlackClient_GetConversations_OngoingVerification struct {
	mock              *MockUnderlyingSlackClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockUnderlyingSlackClient_GetConversations_OngoingVerification) GetCapturedArguments() *slack.GetConversationsParameters {
	conversationParams := c.GetAllCapturedArguments()
	return conversationParams[len(conversationParams)-1]
}

func (c *MockUnderlyingSlackClient_GetConversations_OngoingVerification) GetAllCapturedArguments() (_param0 []*slack.GetConversationsParameters) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]*slack.GetConversationsParameters, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(*slack.GetConversationsParameters)
		}
	}
	return
}

func (verifier *VerifierMockUnderlyingSlackClient) PostMessage(channel string, text string, parameters slack.PostMessageParameters) *MockUnderlyingSlackClient_PostMessage_OngoingVerification {
	params := []pegomock.Param{channel, text, parameters}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "PostMessage", params, verifier.timeout)
	return &MockUnderlyingSlackClient_PostMessage_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockUnderlyingSlackClient_PostMessage_OngoingVerification struct {
	mock              *MockUnderlyingSlackClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockUnderlyingSlackClient_PostMessage_OngoingVerification) GetCapturedArguments() (string, string, slack.PostMessageParameters) {
	channel, text, parameters := c.GetAllCapturedArguments()
	return channel[len(channel)-1], text[len(text)-1], parameters[len(parameters)-1]
}

func (c *MockUnderlyingSlackClient_PostMessage_OngoingVerification) GetAllCapturedArguments() (_param0 []string, _param1 []string, _param2 []slack.PostMessageParameters) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(c.methodInvocations))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
		_param1 = make([]string, len(c.methodInvocations))
		for u, param := range params[1] {
			_param1[u] = param.(string)
		}
		_param2 = make([]slack.PostMessageParameters, len(c.methodInvocations))
		for u, param := range params[2] {
			_param2[u] = param.(slack.PostMessageParameters)
		}
	}
	return
}
