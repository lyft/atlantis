// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"reflect"

	"github.com/petergtz/pegomock"

	webhooks "github.com/runatlantis/atlantis/server/legacy/events/webhooks"
)

func AnyWebhooksApplyResult() webhooks.ApplyResult {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(webhooks.ApplyResult))(nil)).Elem()))
	var nullValue webhooks.ApplyResult
	return nullValue
}

func EqWebhooksApplyResult(value webhooks.ApplyResult) webhooks.ApplyResult {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue webhooks.ApplyResult
	return nullValue
}

func NotEqWebhooksApplyResult(value webhooks.ApplyResult) webhooks.ApplyResult {
	pegomock.RegisterMatcher(&pegomock.NotEqMatcher{Value: value})
	var nullValue webhooks.ApplyResult
	return nullValue
}

func WebhooksApplyResultThat(matcher pegomock.ArgumentMatcher) webhooks.ApplyResult {
	pegomock.RegisterMatcher(matcher)
	var nullValue webhooks.ApplyResult
	return nullValue
}
