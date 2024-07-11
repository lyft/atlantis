// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	models "github.com/runatlantis/atlantis/server/models"
)

func AnyMapOfStringToModelsProjectLock() map[string]models.ProjectLock {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(map[string]models.ProjectLock))(nil)).Elem()))
	var nullValue map[string]models.ProjectLock
	return nullValue
}

func EqMapOfStringToModelsProjectLock(value map[string]models.ProjectLock) map[string]models.ProjectLock {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue map[string]models.ProjectLock
	return nullValue
}

func NotEqMapOfStringToModelsProjectLock(value map[string]models.ProjectLock) map[string]models.ProjectLock {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue map[string]models.ProjectLock
	return nullValue
}

func MapOfStringToModelsProjectLockThat(matcher pegomock.ArgumentMatcher) map[string]models.ProjectLock {
	pegomock.RegisterMatcher(matcher)
	var nullValue map[string]models.ProjectLock
	return nullValue
}
