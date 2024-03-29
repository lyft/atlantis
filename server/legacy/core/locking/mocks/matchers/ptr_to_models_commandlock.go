// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	command "github.com/runatlantis/atlantis/server/legacy/events/command"
)

func AnyPtrToModelsCommandLock() *command.Lock {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(*command.Lock))(nil)).Elem()))
	var nullValue *command.Lock
	return nullValue
}

func EqPtrToModelsCommandLock(value *command.Lock) *command.Lock {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue *command.Lock
	return nullValue
}

func NotEqPtrToModelsCommandLock(value *command.Lock) *command.Lock {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue *command.Lock
	return nullValue
}

func PtrToModelsCommandLockThat(matcher pegomock.ArgumentMatcher) *command.Lock {
	pegomock.RegisterMatcher(matcher)
	var nullValue *command.Lock
	return nullValue
}
