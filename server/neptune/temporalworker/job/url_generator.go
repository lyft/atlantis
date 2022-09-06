package job

import (
	"fmt"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

type UrlGenerator struct {
	// Underlying is the router that the routes have been constructed on.
	Underlying *mux.Router
	// ProjectJobsViewRouteName is the named route for the projects active jobs
	ProjectJobsViewRouteName string
	// AtlantisURL is the fully qualified URL that Atlantis is
	// accessible from externally.
	AtlantisURL *url.URL
}

func (r *UrlGenerator) GenerateJobURL(jobId string) (string, error) {
	if jobId == "" {
		return "", fmt.Errorf("no job id")
	}
	jobURL, err := r.Underlying.Get((r.ProjectJobsViewRouteName)).URL(
		"job-id", jobId,
	)
	if err != nil {
		return "", errors.Wrapf(err, "creating job url for %s", jobId)
	}

	return r.AtlantisURL.String() + jobURL.String(), nil
}
