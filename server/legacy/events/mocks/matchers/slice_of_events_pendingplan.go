// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	events "github.com/runatlantis/atlantis/server/legacy/events"
)

func AnySliceOfEventsPendingPlan() []events.PendingPlan {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*([]events.PendingPlan))(nil)).Elem()))
	var nullValue []events.PendingPlan
	return nullValue
}

func EqSliceOfEventsPendingPlan(value []events.PendingPlan) []events.PendingPlan {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue []events.PendingPlan
	return nullValue
}

func NotEqSliceOfEventsPendingPlan(value []events.PendingPlan) []events.PendingPlan {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue []events.PendingPlan
	return nullValue
}

func SliceOfEventsPendingPlanThat(matcher pegomock.ArgumentMatcher) []events.PendingPlan {
	pegomock.RegisterMatcher(matcher)
	var nullValue []events.PendingPlan
	return nullValue
}
