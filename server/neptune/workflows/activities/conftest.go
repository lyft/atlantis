package activities

import (
	"context"
	"github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/command"
)

type conftestActivity struct {
	DefaultConftestVersion *version.Version
	ConftestClient         *command.AsyncClient
	StreamHandler          streamer
}

type ConftestRequest struct {
	Args        []command.Argument
	DynamicEnvs []EnvVar
	JobID       string
	Version     string
	Path        string
	ShowFile    string
}

type ConftestResponse struct {
	Output string
}

func (c *conftestActivity) Conftest(ctx context.Context, request ConftestRequest) (ConftestResponse, error) {
	// TODO: Implement conftest activity
	return ConftestResponse{}, nil
}
