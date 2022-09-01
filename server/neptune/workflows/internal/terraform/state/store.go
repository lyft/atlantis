package state

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
)

type UpdateNotifier func(state *Workflow) error

type WorkflowStore struct {
	state              *Workflow
	Notifier           UpdateNotifier
	OutputURLGenerator *OutputURLGenerator
}

func (s *WorkflowStore) InitPlanJob(jobID fmt.Stringer, serverURL *url.URL) error {
	outputURL, err := s.OutputURLGenerator.Generate(jobID, serverURL)

	if err != nil {
		return errors.Wrap(err, "generating url for plan job")
	}
	s.state.Plan = &Job{
		Output: &JobOutput{
			URL: outputURL,
		},
		Status: InProgressJobStatus,
	}

	return s.Notifier(s.state)
}

func (s *WorkflowStore) InitApplyJob(jobID fmt.Stringer, serverURL *url.URL) error {
	outputURL, err := s.OutputURLGenerator.Generate(jobID, serverURL)

	if err != nil {
		return errors.Wrap(err, "generating url for plan job")
	}
	s.state.Plan = &Job{
		Output: &JobOutput{
			URL: outputURL,
		},
		Status: InProgressJobStatus,
	}

	return s.Notifier(s.state)
}

func (s *WorkflowStore) UpdatePlanJobWithStatus(status JobStatus) error {
	s.state.Plan.Status = status
	return s.Notifier(s.state)
}

func (s *WorkflowStore) UpdateApplyJobWithStatus(status JobStatus) error {
	s.state.Apply.Status = status
	return s.Notifier(s.state)
}
