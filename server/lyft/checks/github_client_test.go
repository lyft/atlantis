package checks

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func TestGithubClient_PendingCommitStatusWhenUsingChecksAPI(t *testing.T) {
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
		  }
		],
		"sha": "sha",
		"total_count": 1,
		"commit_url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		"url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e/status"
	  }
	`

	statusName := "atlantis/apply"
	pendingStatusResolved := false

	testServer := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			// Update Status
			case "/api/v3/repos/owner/repo/statuses/sha":
				body, err := ioutil.ReadAll(r.Body)
				assert.NoError(t, err)

				m := make(map[string]interface{})
				err = json.Unmarshal(body, &m)
				assert.NoError(t, err)

				if m["context"] == statusName {
					pendingStatusResolved = true
				}

			// Get statuses
			case "/api/v3/repos/owner/repo/commits/sha/status?per_page=100":
				_, err := w.Write([]byte(fmt.Sprintf(listStatusesResp, statusName)))
				assert.NoError(t, err)

			// List checkruns
			case "/api/v3/repos/owner/repo/commits/sha/check-runs?per_page=100":
				_, err := w.Write([]byte(listCheckRunResp))
				assert.NoError(t, err)

			// Create checkrun
			case "/api/v3/repos/owner/repo/check-runs":

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
		FeatureAllocator: &mockFeatureAllocator{},
		Logger:           logging.NewNoopCtxLogger(t),
	}

	checksClientWrapper.UpdateStatus(context.TODO(), types.UpdateStatusRequest{
		StatusName: statusName,
		Ref:        "sha",
		Repo: models.Repo{
			Owner: "owner",
			Name:  "repo",
		},
		State: models.SuccessCommitStatus,
	})

	assert.True(t, pendingStatusResolved)
}

func TestGithubClient_NoStatusUpdateIfSameState(t *testing.T) {
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
		  }
		],
		"sha": "sha",
		"total_count": 1,
		"commit_url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e",
		"url": "https://api.github.com/repos/octocat/Hello-World/6dcb09b5b57875f334f61aebed695e2e4193db5e/status"
	  }
	`

	statusName := "atlantis/apply"
	updateStatusReqReceived := false

	testServer := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			// Update Status
			case "/api/v3/repos/owner/repo/statuses/sha":
				updateStatusReqReceived = true
				body, err := ioutil.ReadAll(r.Body)
				assert.NoError(t, err)

				m := make(map[string]interface{})
				err = json.Unmarshal(body, &m)
				assert.NoError(t, err)
			// Get statuses
			case "/api/v3/repos/owner/repo/commits/sha/status?per_page=100":
				_, err := w.Write([]byte(fmt.Sprintf(listStatusesResp, statusName)))
				assert.NoError(t, err)

			// List checkruns
			case "/api/v3/repos/owner/repo/commits/sha/check-runs?per_page=100":
				_, err := w.Write([]byte(listCheckRunResp))
				assert.NoError(t, err)

			// Create checkrun
			case "/api/v3/repos/owner/repo/check-runs":

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
		FeatureAllocator: &mockFeatureAllocator{},
		Logger:           logging.NewNoopCtxLogger(t),
	}

	checksClientWrapper.UpdateStatus(context.TODO(), types.UpdateStatusRequest{
		StatusName: statusName,
		Ref:        "sha",
		Repo: models.Repo{
			Owner: "owner",
			Name:  "repo",
		},
		State: models.PendingCommitStatus,
	})

	assert.False(t, updateStatusReqReceived)
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

type mockFeatureAllocator struct{}

func (c *mockFeatureAllocator) ShouldAllocate(featureID feature.Name, fullRepoName string) (bool, error) {
	return true, nil
}
