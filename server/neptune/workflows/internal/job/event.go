package job

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// State represent current state of the job
// Job can be in 3 states:
//   * RUNNING - when the job is initiated
//   * FAILURE - when the job fails the execution
//   * SUCCESS - when the job runs successfully
type State string

// Type represent the type of the job
// Currently only apply is supported
type Type string

const (
	Running State = "RUNNING"
	Success State = "SUCCESS"
	Failure State = "FAILURE"

	ApplyJob Type = "APPLY"
)

// AtlantisJobEvent contains metadata of the state of the AtlantisJobType command
type Event struct {
	Version        int    `json:"version"`
	ID             string `json:"id"`
	State          State  `json:"state"`
	JobType        Type   `json:"job_type"`
	Revision       string `json:"revision"`
	Repository     string `json:"repository"`
	PullNumber     int    `json:"pull_number"`
	Environment    string `json:"environment"`
	InitiatingUser string `json:"initiating_user"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	ForceApply     bool   `json:"force_apply"`

	// Service name in the manifest.yaml
	Project string `json:"project"`
	// ProjectName in the atlantis.yaml
	RootName string `json:"root_name"`

	// Currently we do not track approvers metadata.
	ApprovedBy   string `json:"approved_by"`
	ApprovedTime string `json:"approved_time"`
}

func (a *Event) Marshal() ([]byte, error) {
	eventPayload, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling atlantis job event")
	}

	return eventPayload, nil
}
