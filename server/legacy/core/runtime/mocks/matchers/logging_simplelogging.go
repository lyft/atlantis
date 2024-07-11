// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"
	logging "github.com/runatlantis/atlantis/server/logging"
)

func AnyLoggingSimpleLogging() logging.SimpleLogging {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(logging.SimpleLogging))(nil)).Elem()))
	var nullValue logging.SimpleLogging
	return nullValue
}

func EqLoggingSimpleLogging(value logging.SimpleLogging) logging.SimpleLogging {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue logging.SimpleLogging
	return nullValue
}
