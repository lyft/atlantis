// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	io "io"
)

func AnyIoReader() io.Reader {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(io.Reader))(nil)).Elem()))
	var nullValue io.Reader
	return nullValue
}

func EqIoReader(value io.Reader) io.Reader {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue io.Reader
	return nullValue
}

func NotEqIoReader(value io.Reader) io.Reader {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue io.Reader
	return nullValue
}

func IoReaderThat(matcher pegomock.ArgumentMatcher) io.Reader {
	pegomock.RegisterMatcher(matcher)
	var nullValue io.Reader
	return nullValue
}
