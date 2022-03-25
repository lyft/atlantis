// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/runatlantis/atlantis/server/events (interfaces: GitlabMergeRequestGetter)

package mocks

import (
	pegomock "github.com/petergtz/pegomock"
	go_gitlab "github.com/xanzy/go-gitlab"
	"reflect"
	"time"
)

type MockGitlabMergeRequestGetter struct {
	fail func(message string, callerSkip ...int)
}

func NewMockGitlabMergeRequestGetter(options ...pegomock.Option) *MockGitlabMergeRequestGetter {
	mock := &MockGitlabMergeRequestGetter{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockGitlabMergeRequestGetter) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockGitlabMergeRequestGetter) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockGitlabMergeRequestGetter) GetMergeRequest(repoFullName string, pullNum int) (*go_gitlab.MergeRequest, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockGitlabMergeRequestGetter().")
	}
	params := []pegomock.Param{repoFullName, pullNum}
	result := pegomock.GetGenericMockFrom(mock).Invoke("GetMergeRequest", params, []reflect.Type{reflect.TypeOf((**go_gitlab.MergeRequest)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *go_gitlab.MergeRequest
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*go_gitlab.MergeRequest)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockGitlabMergeRequestGetter) VerifyWasCalledOnce() *VerifierMockGitlabMergeRequestGetter {
	return &VerifierMockGitlabMergeRequestGetter{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockGitlabMergeRequestGetter) VerifyWasCalled(invocationCountMatcher pegomock.InvocationCountMatcher) *VerifierMockGitlabMergeRequestGetter {
	return &VerifierMockGitlabMergeRequestGetter{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockGitlabMergeRequestGetter) VerifyWasCalledInOrder(invocationCountMatcher pegomock.InvocationCountMatcher, inOrderContext *pegomock.InOrderContext) *VerifierMockGitlabMergeRequestGetter {
	return &VerifierMockGitlabMergeRequestGetter{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockGitlabMergeRequestGetter) VerifyWasCalledEventually(invocationCountMatcher pegomock.InvocationCountMatcher, timeout time.Duration) *VerifierMockGitlabMergeRequestGetter {
	return &VerifierMockGitlabMergeRequestGetter{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockGitlabMergeRequestGetter struct {
	mock                   *MockGitlabMergeRequestGetter
	invocationCountMatcher pegomock.InvocationCountMatcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockGitlabMergeRequestGetter) GetMergeRequest(repoFullName string, pullNum int) *MockGitlabMergeRequestGetter_GetMergeRequest_OngoingVerification {
	params := []pegomock.Param{repoFullName, pullNum}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "GetMergeRequest", params, verifier.timeout)
	return &MockGitlabMergeRequestGetter_GetMergeRequest_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockGitlabMergeRequestGetter_GetMergeRequest_OngoingVerification struct {
	mock              *MockGitlabMergeRequestGetter
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockGitlabMergeRequestGetter_GetMergeRequest_OngoingVerification) GetCapturedArguments() (string, int) {
	repoFullName, pullNum := c.GetAllCapturedArguments()
	return repoFullName[len(repoFullName)-1], pullNum[len(pullNum)-1]
}

func (c *MockGitlabMergeRequestGetter_GetMergeRequest_OngoingVerification) GetAllCapturedArguments() (_param0 []string, _param1 []int) {
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
