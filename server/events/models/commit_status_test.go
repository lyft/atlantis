package models_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
)

func TestStatus_String(t *testing.T) {
	cases := map[models.VcsStatus]string{
		models.PendingVcsStatus: "pending",
		models.SuccessVcsStatus: "success",
		models.FailedVcsStatus:  "failed",
	}
	for k, v := range cases {
		Equals(t, v, k.String())
	}
}
