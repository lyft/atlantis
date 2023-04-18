package terraform

import "github.com/runatlantis/atlantis/server/core/config/valid"

type Response struct {
	FailedPolicies map[string]valid.PolicySet
}
