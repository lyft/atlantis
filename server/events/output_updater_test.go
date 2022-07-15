package events

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/checks"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/runatlantis/atlantis/server/vcs/markdown"
	"github.com/stretchr/testify/assert"
)

// disableSSLVerification disables ssl verification for the global http client
// and returns a function to be called in a defer that will re-enable it.
func disableSSLVerification() func() {
	orig := http.DefaultTransport.(*http.Transport).TLSClientConfig
	// nolint: gosec
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = orig
	}
}

type StatusType int

const (
	CommitStatus StatusType = iota
	ChecksStatus
)

func TestUpdateOutput(t *testing.T) {
	repo := models.Repo{
		Owner: "owner",
		Name:  "repo",
		VCSHost: models.VCSHost{
			Type: models.Github,
		},
	}

	listCheckRunRespFormat := `
	{
		"total_count": 1,
		"check_runs": [
		]
	  }
	`

	listStatusesResp := `
	{
		"state": "pending",
		"statuses": [
		  {
			"context": "%s",
			"state": "pending",
			"created_at": "2012-07-20T01:19:13Z",
			"updated_at": "2012-07-20T01:19:13Z"
		  },
		  {
			"context": "%s",
			"state": "pending",
			"created_at": "2012-07-20T01:19:13Z",
			"updated_at": "2012-07-20T01:19:13Z"
		  }
		],
		"sha": "sha",
		"total_count": 1,
		"commit_url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		"url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e/status"
	  }
	`

	cases := []struct {
		statusNames   []string
		desription    string
		checksEnabled bool
		expType       StatusType
	}{
		{
			statusNames:   []string{"atlantis/plan", "atlantis/apply"},
			desription:    "all atlantis statuses when checks is enabled",
			checksEnabled: true,
			expType:       CommitStatus,
		},
		{
			statusNames:   []string{"terraform-fmt", "terraform-checks"},
			desription:    "no atlantis status when checks is enabled",
			checksEnabled: true,
			expType:       ChecksStatus,
		},
		{
			statusNames:   []string{"atlantis/plan", "terraform-fmt"},
			desription:    "at least one atlantis status when checks is enabled",
			checksEnabled: true,
			expType:       CommitStatus,
		},
		{
			statusNames:   []string{"terraform-checks", "terraform-fmt"},
			desription:    "no atlantis status when checks is disabled",
			checksEnabled: false,
			expType:       CommitStatus,
		},
	}

	for _, c := range cases {
		t.Run(c.desription, func(t *testing.T) {

			var callType StatusType
			testServer := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.RequestURI {

					// list checkruns
					case "/api/v3/repos/owner/repo/commits/sha/check-runs?per_page=100":
						_, err := w.Write([]byte(listCheckRunRespFormat))
						assert.NoError(t, err)

					// Create status
					case "/api/v3/repos/owner/repo/issues/0/comments":
						callType = CommitStatus
						w.WriteHeader(http.StatusOK)

					case "/api/v3/repos/owner/repo/check-runs":
						callType = ChecksStatus
						w.WriteHeader(http.StatusOK)

					// Get statuses
					case "/api/v3/repos/owner/repo/commits/sha/status?per_page=100":
						_, err := w.Write([]byte(fmt.Sprintf(listStatusesResp, c.statusNames[0], c.statusNames[1])))
						assert.NoError(t, err)

					default:
						t.Errorf("got unexpected request at %q", r.RequestURI)
						http.Error(w, "not found", http.StatusNotFound)
						return
					}
				}))

			testServerURL, err := url.Parse(testServer.URL)
			assert.NoError(t, err)

			mergeabilityChecker := vcs.NewPullMergeabilityChecker("atlantis")
			client, err := vcs.NewGithubClient(testServerURL.Host, &vcs.GithubUserCredentials{"user", "pass"}, logging.NewNoopCtxLogger(t), mergeabilityChecker)

			checksClientWrapper := checks.ChecksClientWrapper{
				GithubClient:     client,
				FeatureAllocator: &mockFeatureAllocator{shouldAllocate: c.checksEnabled},
				Logger:           logging.NewNoopCtxLogger(t),
			}
			assert.NoError(t, err)

			defer disableSSLVerification()()

			outputUpdater := FeatureAwareChecksOutputUpdater{
				PullOutputUpdater: PullOutputUpdater{
					VCSClient:        &checksClientWrapper,
					MarkdownRenderer: &markdown.Renderer{},
				},
				ChecksOutputUpdater: ChecksOutputUpdater{
					VCSClient:        &checksClientWrapper,
					MarkdownRenderer: &markdown.Renderer{},
					TitleBuilder:     vcs.StatusTitleBuilder{TitlePrefix: "atlantis"},
				},
				FeatureAllocator: &mockFeatureAllocator{c.checksEnabled},
				Logger:           logging.NewNoopCtxLogger(t),
				GithubClient:     *client,
			}

			outputUpdater.UpdateOutput(&command.Context{
				HeadRepo: repo,
				Pull: models.PullRequest{
					HeadCommit: "sha",
					BaseRepo:   repo,
				},
				RequestCtx: context.TODO(),
			}, PolicyCheckCommand{}, command.Result{
				ProjectResults: []command.ProjectResult{
					{
						Command:      command.PolicyCheck,
						ApplySuccess: "Apply Success",
					},
				},
			})

			assert.Equal(t, c.expType, callType)
		})

	}

}

type mockFeatureAllocator struct {
	shouldAllocate bool
}

func (m *mockFeatureAllocator) ShouldAllocate(featureID feature.Name, fullRepoName string) (bool, error) {
	return m.shouldAllocate, nil
}
