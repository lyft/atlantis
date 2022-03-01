// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	github "github.com/google/go-github/v31/github"
)

func AnySliceOfPtrToGithubRepoStatus() []*github.RepoStatus {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*([]*github.RepoStatus))(nil)).Elem()))
	var nullValue []*github.RepoStatus
	return nullValue
}

func EqSliceOfPtrToGithubRepoStatus(value []*github.RepoStatus) []*github.RepoStatus {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue []*github.RepoStatus
	return nullValue
}

func NotEqSliceOfPtrToGithubRepoStatus(value []*github.RepoStatus) []*github.RepoStatus {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue []*github.RepoStatus
	return nullValue
}

func SliceOfPtrToGithubRepoStatusThat(matcher pegomock.ArgumentMatcher) []*github.RepoStatus {
	pegomock.RegisterMatcher(matcher)
	var nullValue []*github.RepoStatus
	return nullValue
}
