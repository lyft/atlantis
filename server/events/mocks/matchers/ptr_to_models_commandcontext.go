// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	models "github.com/runatlantis/atlantis/server/events/models"
	"reflect"
)

func AnyPtrToModelsCommandContext() *models.CommandContext {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(*models.CommandContext))(nil)).Elem()))
	var nullValue *models.CommandContext
	return nullValue
}

func EqPtrToModelsCommandContext(value *models.CommandContext) *models.CommandContext {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue *models.CommandContext
	return nullValue
}
