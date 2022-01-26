// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	"reflect"

	models "github.com/runatlantis/atlantis/server/events/models"
)

func AnyModelsPullContext() models.PullContext {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(models.PullContext))(nil)).Elem()))
	var nullValue models.PullContext
	return nullValue
}

func EqModelsPullContext(value models.PullContext) models.PullContext {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue models.PullContext
	return nullValue
}

func NotEqModelsPullContext(value models.PullContext) models.PullContext {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue models.PullContext
	return nullValue
}

func ModelsPullContextThat(matcher pegomock.ArgumentMatcher) models.PullContext {
	pegomock.RegisterMatcher(matcher)
	var nullValue models.PullContext
	return nullValue
}
