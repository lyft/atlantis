// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"
	"github.com/petergtz/pegomock"
	helpers "github.com/runatlantis/atlantis/server/core/terraform/helpers"
)

func AnyRecvChanOfTerraformLine() <-chan helpers.Line {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(<-chan helpers.Line))(nil)).Elem()))
	var nullValue <-chan helpers.Line
	return nullValue
}

func EqRecvChanOfTerraformLine(value <-chan helpers.Line) <-chan helpers.Line {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue <-chan helpers.Line
	return nullValue
}
