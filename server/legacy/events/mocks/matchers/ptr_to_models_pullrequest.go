// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	models "github.com/runatlantis/atlantis/server/models"
)

func AnyPtrToModelsPullRequest() *models.PullRequest {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(*models.PullRequest))(nil)).Elem()))
	var nullValue *models.PullRequest
	return nullValue
}

func EqPtrToModelsPullRequest(value *models.PullRequest) *models.PullRequest {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue *models.PullRequest
	return nullValue
}

func NotEqPtrToModelsPullRequest(value *models.PullRequest) *models.PullRequest {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue *models.PullRequest
	return nullValue
}

func PtrToModelsPullRequestThat(matcher pegomock.ArgumentMatcher) *models.PullRequest {
	pegomock.RegisterMatcher(matcher)
	var nullValue *models.PullRequest
	return nullValue
}
