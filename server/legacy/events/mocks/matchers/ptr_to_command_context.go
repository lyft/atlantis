// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	command "github.com/runatlantis/atlantis/server/legacy/events/command"
)

func AnyPtrToCommandContext() *command.Context {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(*command.Context))(nil)).Elem()))
	var nullValue *command.Context
	return nullValue
}

func EqPtrToCommandContext(value *command.Context) *command.Context {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue *command.Context
	return nullValue
}

func NotEqPtrToCommandContext(value *command.Context) *command.Context {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue *command.Context
	return nullValue
}

func PtrToCommandContextThat(matcher pegomock.ArgumentMatcher) *command.Context {
	pegomock.RegisterMatcher(matcher)
	var nullValue *command.Context
	return nullValue
}
