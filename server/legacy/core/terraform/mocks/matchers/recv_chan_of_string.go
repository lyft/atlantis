// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	"reflect"
)

func AnyRecvChanOfString() <-chan string {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(<-chan string))(nil)).Elem()))
	var nullValue <-chan string
	return nullValue
}

func EqRecvChanOfString(value <-chan string) <-chan string {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue <-chan string
	return nullValue
}