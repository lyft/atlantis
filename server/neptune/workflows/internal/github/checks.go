package github

import (
	"fmt"

	"github.com/google/go-github/v45/github"
)

type CheckRunState string
type CheckRunConclusion string

type CheckRunAction interface {
	ToGithubAction() *github.CheckRunAction
}

type PlanReviewActionType string

type PlanReviewAction struct {
	ActionType PlanReviewActionType
}

func (a PlanReviewAction) ToGithubAction() *github.CheckRunAction {
	return &github.CheckRunAction{
		Label:       string(a.ActionType),
		Description: fmt.Sprintf("%s this plan to proceed to the apply", string(a.ActionType)),

		// we encode the action type as the id since there's a 20 char limit anyways
		// and we can use the check run external id to map to the correct workflow
		Identifier: string(a.ActionType),
	}
}

const (
	CheckRunSuccess  CheckRunConclusion = "success"
	CheckRunFailure  CheckRunConclusion = "failure"
	CheckRunComplete CheckRunState      = "completed"
	CheckRunPending  CheckRunState      = "in_progress"
	CheckRunQueued   CheckRunState      = "queued"

	Approved PlanReviewActionType = "approved"
	Reject   PlanReviewActionType = "reject"
)
