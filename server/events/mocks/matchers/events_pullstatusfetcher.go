// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	"reflect"

	events "github.com/runatlantis/atlantis/server/events"
)

func AnyEventsPullStatusFetcher() events.PullStatusFetcher {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(events.PullStatusFetcher))(nil)).Elem()))
	var nullValue events.PullStatusFetcher
	return nullValue
}

func EqEventsPullStatusFetcher(value events.PullStatusFetcher) events.PullStatusFetcher {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue events.PullStatusFetcher
	return nullValue
}

func NotEqEventsPullStatusFetcher(value events.PullStatusFetcher) events.PullStatusFetcher {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue events.PullStatusFetcher
	return nullValue
}

func EventsPullStatusFetcherThat(matcher pegomock.ArgumentMatcher) events.PullStatusFetcher {
	pegomock.RegisterMatcher(matcher)
	var nullValue events.PullStatusFetcher
	return nullValue
}
