// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"
	"github.com/petergtz/pegomock"
	
)

func AnyChanOfString() chan string {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(chan string))(nil)).Elem()))
	var nullValue chan string
	return nullValue
}

func EqChanOfString(value chan string) chan string {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue chan string
	return nullValue
}
