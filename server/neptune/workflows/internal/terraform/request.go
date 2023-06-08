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
	WorkflowMode terraform.WorkflowMode
}

const (
	PlanRejectedErrorType = "PlanRejectedError"
	ValidationErrorType   = "ValidationErrorType"
	UpdateJobErrorType    = "UpdateJobError"
	UnknownErrorType      = "UnknownError"
	SchedulingError       = "SchedulingError"
)

type ExternalError struct {
	ErrType string
}

func (e ExternalError) GetExternalType() string {
	return e.ErrType
}

type PlanRejectedError struct {
	Err error
	ExternalError
}

func (e PlanRejectedError) Error() string {
	return e.Err.Error()
}

func newPlanRejectedError() PlanRejectedError {
	return PlanRejectedError{
		Err:           fmt.Errorf("plan is rejected, apply cannot proceed"),
		ExternalError: ExternalError{ErrType: PlanRejectedErrorType},
	}
}

type UpdateJobError struct {
	err error
	msg string
	ExternalError
}

func (e UpdateJobError) Error() string {
	return errors.Wrap(e.err, e.msg).Error()
}

func newUpdateJobError(err error, msg string) UpdateJobError {
	return UpdateJobError{
		err:           errors.Wrap(err, msg),
		ExternalError: ExternalError{ErrType: UpdateJobErrorType},
	}
}

type ValidationError struct {
	Err error
	ExternalError
}

func (e ValidationError) Error() string {
	return e.Err.Error()
}

func newValidationError() ValidationError {
	return ValidationError{
		Err:           fmt.Errorf("plan failed validation checks"),
		ExternalError: ExternalError{ErrType: ValidationErrorType},
	}
}
