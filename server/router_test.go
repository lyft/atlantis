package server_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/stretchr/testify/assert"
)

func TestRouter_GenerateLockURL(t *testing.T) {
	cases := []struct {
		AtlantisURL *url.URL
		ExpURL      string
	}{
		{
			&url.URL{
				Scheme: "http",
				Host:   "localhost:4141",
			},
			"http://localhost:4141/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
		},
		{
			&url.URL{
				Scheme: "https",
				Host:   "localhost:4141",
			},
			"https://localhost:4141/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
		},
		{
			&url.URL{
				Scheme: "https",
				Host:   "localhost:4141",
			},
			"https://localhost:4141/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
		},
		{
			&url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/basepath",
			},
			"https://example.com/basepath/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
		},
		{
			&url.URL{
				Scheme: "https",
				Host:   "example.com",
				Path:   "/path/1",
			},
			"https://example.com/path/1/lock?id=lkysow%252Fatlantis-example%252F.%252Fdefault",
		},
	}

	queryParam := "id"
	routeName := "routename"
	underlyingRouter := mux.NewRouter()
	underlyingRouter.HandleFunc("/lock", func(_ http.ResponseWriter, _ *http.Request) {}).Methods(http.MethodGet).Queries(queryParam, "{id}").Name(routeName)

	for _, c := range cases {
		t.Run(c.AtlantisURL.String(), func(t *testing.T) {
			router := &server.Router{
				AtlantisURL:               c.AtlantisURL,
				LockViewRouteIDQueryParam: queryParam,
				LockViewRouteName:         routeName,
				Underlying:                underlyingRouter,
			}
			Equals(t, c.ExpURL, router.GenerateLockURL("lkysow/atlantis-example/./default"))
		})
	}
}

func setupJobsRouter(t *testing.T) *server.Router {
	underlyingRouter := mux.NewRouter()
	underlyingRouter.HandleFunc("/jobs/{job-id}", func(_ http.ResponseWriter, _ *http.Request) {}).Methods(http.MethodGet).Name("project-jobs-detail")

	return &server.Router{
		AtlantisURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:4141",
		},
		Underlying:               underlyingRouter,
		ProjectJobsViewRouteName: "project-jobs-detail",
	}
}

func TestGenerateProjectJobURL_ShouldGenerateURLWhenJobIDSpecified(t *testing.T) {
	router := setupJobsRouter(t)
	jobID := uuid.New().String()
	ctx := command.ProjectContext{
		JobID: jobID,
	}
	expectedURL := fmt.Sprintf("http://localhost:4141/jobs/%s", jobID)
	gotURL, err := router.GenerateProjectJobURL(ctx.JobID)
	Ok(t, err)

	Equals(t, expectedURL, gotURL)
}

func TestGenerateProjectJobURL_ShouldReturnErrorWhenJobIDNotSpecified(t *testing.T) {
	router := setupJobsRouter(t)
	ctx := command.ProjectContext{
		Pull: models.PullRequest{
			BaseRepo: models.Repo{
				Owner: "test-owner",
				Name:  "test-repo",
			},
			Num: 1,
		},
		RepoRelDir: "ops/terraform/",
	}
	expectedErrString := "no job id in ctx"
	gotURL, err := router.GenerateProjectJobURL(ctx.JobID)
	assert.EqualError(t, err, expectedErrString)
	Equals(t, "", gotURL)
}
