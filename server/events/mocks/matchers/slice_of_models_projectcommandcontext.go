// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events/command/project"
)

func AnySliceOfModelsProjectCommandContext() []project.Context {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*([]project.Context))(nil)).Elem()))
	var nullValue []project.Context
	return nullValue
}

func EqSliceOfModelsProjectCommandContext(value []project.Context) []project.Context {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue []project.Context
	return nullValue
}

func NotEqSliceOfModelsProjectCommandContext(value []project.Context) []project.Context {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue []project.Context
	return nullValue
}

func SliceOfModelsProjectCommandContextThat(matcher pegomock.ArgumentMatcher) []project.Context {
	pegomock.RegisterMatcher(matcher)
	var nullValue []project.Context
	return nullValue
}
