package project

import (
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
)

// Result is the result of executing a plan/policy_check/apply for a specific project.
type Result struct {
	Command            command.Name
	RepoRelDir         string
	Workspace          string
	Error              error
	Failure            string
	PlanSuccess        *PlanSuccess
	PolicyCheckSuccess *PolicyCheckSuccess
	ApplySuccess       string
	VersionSuccess     string
	ProjectName        string
}

// CommitStatus returns the vcs commit status of this project result.
func (p Result) CommitStatus() models.CommitStatus {
	if p.Error != nil {
		return models.FailedCommitStatus
	}
	if p.Failure != "" {
		return models.FailedCommitStatus
	}
	return models.SuccessCommitStatus
}

// PlanStatus returns the plan status.
func (p Result) PlanStatus() models.ProjectPlanStatus {
	switch p.Command {

	case command.Plan:
		if p.Error != nil {
			return models.ErroredPlanStatus
		} else if p.Failure != "" {
			return models.ErroredPlanStatus
		}
		return models.PlannedPlanStatus
	case command.PolicyCheck, command.ApprovePolicies:
		if p.Error != nil {
			return models.ErroredPolicyCheckStatus
		} else if p.Failure != "" {
			return models.ErroredPolicyCheckStatus
		}
		return models.PassedPolicyCheckStatus
	case command.Apply:
		if p.Error != nil {
			return models.ErroredApplyStatus
		} else if p.Failure != "" {
			return models.ErroredApplyStatus
		}
		return models.AppliedPlanStatus
	}

	panic("PlanStatus() missing a combination")
}
