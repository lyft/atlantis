// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	types "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func AnyTypesMessage() types.Message {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(types.Message))(nil)).Elem()))
	var nullValue types.Message
	return nullValue
}

func EqTypesMessage(value types.Message) types.Message {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue types.Message
	return nullValue
}

func NotEqTypesMessage(value types.Message) types.Message {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue types.Message
	return nullValue
}

func TypesMessageThat(matcher pegomock.ArgumentMatcher) types.Message {
	pegomock.RegisterMatcher(matcher)
	var nullValue types.Message
	return nullValue
}
