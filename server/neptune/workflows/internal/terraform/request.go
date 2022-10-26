package terraform

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type Request struct {
	Root         terraform.Root
	Repo         github.Repo
	DeploymentID string
	Revision     string
}

type ClientError struct {
	Err error
}

func (e *ClientError) Error() string {
	return e.Err.Error()
}

type PlanRejectedError struct {
	Err error
}

func (e PlanRejectedError) Error() string {
	return e.Err.Error()
}

func newPlanRejectedError() PlanRejectedError {
	return PlanRejectedError{
		Err: fmt.Errorf("plan is rejected, apply cannot proceed"),
	}
}

type UpdateJobError struct {
	err error
	msg string
}

func (e UpdateJobError) Error() string {
	return errors.Wrap(e.err, e.msg).Error()
}

func newUpdateJobError(err error, msg string) UpdateJobError {
	return UpdateJobError{
		err: errors.Wrap(err, msg),
	}
}
