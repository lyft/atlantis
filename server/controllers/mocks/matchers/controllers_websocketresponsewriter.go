// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	"reflect"

	controllers "github.com/runatlantis/atlantis/server/controllers"
)

func AnyControllersWebsocketResponseWriter() controllers.WebsocketResponseWriter {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(controllers.WebsocketResponseWriter))(nil)).Elem()))
	var nullValue controllers.WebsocketResponseWriter
	return nullValue
}

func EqControllersWebsocketResponseWriter(value controllers.WebsocketResponseWriter) controllers.WebsocketResponseWriter {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue controllers.WebsocketResponseWriter
	return nullValue
}

func NotEqControllersWebsocketResponseWriter(value controllers.WebsocketResponseWriter) controllers.WebsocketResponseWriter {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue controllers.WebsocketResponseWriter
	return nullValue
}

func ControllersWebsocketResponseWriterThat(matcher pegomock.ArgumentMatcher) controllers.WebsocketResponseWriter {
	pegomock.RegisterMatcher(matcher)
	var nullValue controllers.WebsocketResponseWriter
	return nullValue
}
