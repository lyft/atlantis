// Code generated by pegomock. DO NOT EDIT.
package matchers

import (
	"github.com/petergtz/pegomock"
	valid "github.com/runatlantis/atlantis/server/core/config/valid"
	"reflect"
)

func AnyValidPolicySet() valid.PolicySet {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((*(valid.PolicySet))(nil)).Elem()))
	var nullValue valid.PolicySet
	return nullValue
}

func EqValidPolicySet(value valid.PolicySet) valid.PolicySet {
	pegomock.RegisterMatcher(&pegomock.EqMatcher{Value: value})
	var nullValue valid.PolicySet
	return nullValue
}
