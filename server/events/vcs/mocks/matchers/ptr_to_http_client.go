// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	http "net/http"
)

func AnyPtrToHTTPClient() *http.Client {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(*http.Client))(nil)).Elem()))
	var nullValue *http.Client
	return nullValue
}

func EqPtrToHTTPClient(value *http.Client) *http.Client {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue *http.Client
	return nullValue
}

func NotEqPtrToHTTPClient(value *http.Client) *http.Client {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue *http.Client
	return nullValue
}

func PtrToHTTPClientThat(matcher pegomock.ArgumentMatcher) *http.Client {
	pegomock.RegisterMatcher(matcher)
	var nullValue *http.Client
	return nullValue
}
