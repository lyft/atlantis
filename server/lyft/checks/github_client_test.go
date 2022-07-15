package checks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/vcs"
	"github.com/runatlantis/atlantis/server/events/vcs/types"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/stretchr/testify/assert"
)

type StatusType int

const (
	CommitStatus StatusType = iota
	ChecksStatus
)

func TestGithubClient_UpdateStatus(t *testing.T) {
	listCheckRunResp := `
	{
		"total_count": 0,
		"check_runs": []
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
		statusNames             []string
		desription              string
		checksEnabled           bool
		numCallsGetRepoStatuses int
		expStatusType           StatusType
		// bool flag to enable github check determined by output updater before call to client.UpdateStatus
		updateReqEnableGithubChecks bool
	}{
		{
			statusNames:             []string{"atlantis/plan", "atlantis/apply"},
			desription:              "all atlantis statuses when checks is enabled",
			checksEnabled:           true,
			expStatusType:           CommitStatus,
			numCallsGetRepoStatuses: 1,
		},
		{
			statusNames:             []string{"terraform-fmt", "terraform-checks"},
			desription:              "no atlantis status when checks is enabled",
			checksEnabled:           true,
			expStatusType:           ChecksStatus,
			numCallsGetRepoStatuses: 1,
		},
		{
			desription:                  "github checks is enabled in the update request",
			checksEnabled:               true,
			expStatusType:               ChecksStatus,
			numCallsGetRepoStatuses:     0, // No need to get repo statuses if updateReq.useGithubChecks is true
			updateReqEnableGithubChecks: true,
		},
		{
			statusNames:             []string{"atlantis/plan", "terraform-fmt"},
			desription:              "at least one atlantis status when checks is enabled",
			checksEnabled:           true,
			expStatusType:           CommitStatus,
			numCallsGetRepoStatuses: 1,
		},
		{
			statusNames:             []string{"terraform-checks", "terraform-fmt"},
			desription:              "no atlantis status when checks is disabled",
			checksEnabled:           false,
			expStatusType:           CommitStatus,
			numCallsGetRepoStatuses: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.desription, func(t *testing.T) {
			var statusType StatusType
			var numCallsGetRepoStatuses int
			testServer := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.RequestURI {

					// Create status
					case "/api/v3/repos/owner/repo/statuses/sha":
						statusType = CommitStatus
						w.WriteHeader(http.StatusOK)

					case "/api/v3/repos/owner/repo/check-runs":
						statusType = ChecksStatus
						w.WriteHeader(http.StatusOK)

					case "/api/v3/repos/owner/repo/commits/sha/check-runs?per_page=100":
						_, err := w.Write([]byte(listCheckRunResp))
						assert.NoError(t, err)

					// Get statuses
					case "/api/v3/repos/owner/repo/commits/sha/status?per_page=100":
						numCallsGetRepoStatuses += 1
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
			assert.NoError(t, err)

			defer disableSSLVerification()()

			checksClientWrapper := ChecksClientWrapper{
				GithubClient:     client,
				FeatureAllocator: &mockFeatureAllocator{c.checksEnabled},
				Logger:           logging.NewNoopCtxLogger(t),
			}

			checksClientWrapper.UpdateStatus(context.TODO(), types.UpdateStatusRequest{
				StatusName: "anything",
				Ref:        "sha",
				Repo: models.Repo{
					Owner: "owner",
					Name:  "repo",
				},
				State:           models.SuccessCommitStatus,
				UseGithubChecks: c.updateReqEnableGithubChecks,
			})

			assert.Equal(t, c.expStatusType, statusType)
			assert.Equal(t, c.numCallsGetRepoStatuses, numCallsGetRepoStatuses)

		})
	}

}

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

type mockFeatureAllocator struct {
	checksEnabled bool
}

func (c *mockFeatureAllocator) ShouldAllocate(featureID feature.Name, fullRepoName string) (bool, error) {
	return c.checksEnabled, nil
}
