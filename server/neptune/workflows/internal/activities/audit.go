package activities

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

// AtlantisJobState represent current state of the job
// Job can be in 3 states:
//   * RUNNING - when the job is initiated
//   * FAILURE - when the job fails the execution
//   * SUCCESS - when the job runs successfully
type AtlantisJobState string

// AtlantisJobType represent the type of the job
// Currently only apply is supported
type AtlantisJobType string

const (
	AtlantisJobStateRunning AtlantisJobState = "RUNNING"
	AtlantisJobStateSuccess AtlantisJobState = "SUCCESS"
	AtlantisJobStateFailure AtlantisJobState = "FAILURE"

	AtlantisApplyJob AtlantisJobType = "APPLY"
)

// AtlantisJobEvent contains metadata of the state of the AtlantisJobType command
type AtlantisJobEvent struct {
	Version        int              `json:"version"`
	ID             string           `json:"id"`
	State          AtlantisJobState `json:"state"`
	JobType        AtlantisJobType  `json:"job_type"`
	Revision       string           `json:"revision"`
	Repository     string           `json:"repository"`
	PullNumber     int              `json:"pull_number"`
	Environment    string           `json:"environment"`
	InitiatingUser string           `json:"initiating_user"`
	StartTime      string           `json:"start_time"`
	EndTime        string           `json:"end_time"`
	ForceApply     bool             `json:"force_apply"`

	// Service name in the manifest.yaml
	Project string `json:"project"`
	// ProjectName in the atlantis.yaml
	RootName string `json:"root_name"`

	// Currently we do not track approvers metadata.
	// ORCA-954 will implement this feature
	ApprovedBy   string `json:"approved_by"`
	ApprovedTime string `json:"approved_time"`
}

func (a *AtlantisJobEvent) Marshal() ([]byte, error) {
	eventPayload, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling atlantis job event")
	}

	return eventPayload, nil
}

type writer interface {
	Write([]byte) error
}

type auditActivities struct {
	SnsWriter writer
}

type NotifyDeployApiRequest struct {
	DeploymentInfo root.DeploymentInfo
	State          AtlantisJobState
}

func (a *auditActivities) NotifyDeployApi(ctx context.Context, request NotifyDeployApiRequest) error {
	isForceApply := request.DeploymentInfo.Root.Trigger == root.Trigger(request.DeploymentInfo.Root.Trigger)
	atlantisJobEvent := &AtlantisJobEvent{
		Version:        1,
		ID:             request.DeploymentInfo.ID,
		RootName:       request.DeploymentInfo.Root.Name,
		JobType:        AtlantisApplyJob,
		Repository:     request.DeploymentInfo.Repo.GetFullName(),
		Environment:    ctx.Value("environment").(string),
		InitiatingUser: request.DeploymentInfo.User.Username,
		Project:        ctx.Value("service_name").(string),
		ForceApply:     isForceApply,
		StartTime:      strconv.FormatInt(time.Now().Unix(), 10),
		Revision:       request.DeploymentInfo.Revision,
	}

	if request.State == AtlantisJobStateFailure || request.State == AtlantisJobStateSuccess {
		atlantisJobEvent.EndTime = strconv.FormatInt(time.Now().Unix(), 10)
	}

	payload, err := atlantisJobEvent.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshaling atlantis job event")
	}

	if err := a.SnsWriter.Write(payload); err != nil {
		return errors.Wrap(err, "writing to sns topic")
	}

	return nil
}
