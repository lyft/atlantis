package activities

import (
	"context"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/job"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

const (
	EnvironmentTagKey = "environment"
	ProjectTagKey     = "service_name"
)

type writer interface {
	Write([]byte) error
}

type AuditJobRequest struct {
	DeploymentInfo root.DeploymentInfo
	State          job.State
}

type auditActivities struct {
	SnsWriter writer
}

func (a *auditActivities) AuditJob(ctx context.Context, req AuditJobRequest) error {
	isForceApply := req.DeploymentInfo.Root.Trigger == root.ManualTrigger
	atlantisJobEvent := &job.Event{
		Version:        1,
		ID:             req.DeploymentInfo.ID,
		RootName:       req.DeploymentInfo.Root.Name,
		JobType:        job.ApplyJob,
		Repository:     req.DeploymentInfo.Repo.GetFullName(),
		InitiatingUser: req.DeploymentInfo.User.Username,
		ForceApply:     isForceApply,
		StartTime:      strconv.FormatInt(time.Now().Unix(), 10),
		Revision:       req.DeploymentInfo.Revision,
		Project:        req.DeploymentInfo.Tags[ProjectTagKey],
		Environment:    req.DeploymentInfo.Tags[EnvironmentTagKey],
	}

	if req.State == job.Failure || req.State == job.Success {
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
