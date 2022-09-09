package github

import (
	"fmt"

	"github.com/google/go-github/v45/github"
)

type CheckRunState string
type CheckRunConclusion string

type CheckRunAction struct {
	Description string
	Label       string
}

func (a CheckRunAction) ToGithubAction() *github.CheckRunAction {
	return &github.CheckRunAction{
		Label:       a.Label,
		Description: a.Description,

		// we encode the label as the id since there's a 20 char limit anyways
		// and we can use the check run external id to map to the correct workflow
		Identifier: a.Label,
	}
}

func CreatePlanReviewAction(t PlanReviewActionType) CheckRunAction {
	return CheckRunAction{
		Description: fmt.Sprintf("%s this plan to proceed to the apply", string(t)),
		Label:       string(t),
	}

}

type PlanReviewActionType string

const (
	CheckRunSuccess  CheckRunConclusion = "success"
	CheckRunFailure  CheckRunConclusion = "failure"
	CheckRunComplete CheckRunState      = "completed"
	CheckRunPending  CheckRunState      = "in_progress"
	CheckRunQueued   CheckRunState      = "queued"

	Approved PlanReviewActionType = "approved"
	Reject   PlanReviewActionType = "reject"
)
