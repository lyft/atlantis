package server_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-version"
	stats "github.com/lyft/gostats"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/db"
	"github.com/runatlantis/atlantis/server/events/locking"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/runtime"
	"github.com/runatlantis/atlantis/server/events/runtime/policy"
	"github.com/runatlantis/atlantis/server/events/terraform"
	vcsmocks "github.com/runatlantis/atlantis/server/events/vcs/mocks"
	"github.com/runatlantis/atlantis/server/events/webhooks"
	"github.com/runatlantis/atlantis/server/events/yaml"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

type NoopTFDownloader struct{}

func (m *NoopTFDownloader) GetFile(dst, src string, opts ...getter.ClientOption) error {
	return nil
}

func (m *NoopTFDownloader) GetAny(dst, src string, opts ...getter.ClientOption) error {
	return nil
}

func TestGitHubWorkflow(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// Ensure we have >= TF 0.12 locally.
	ensureRunning012(t)

	cases := []struct {
		Description string
		// RepoDir is relative to testfixtures/test-repos.
		RepoDir string
		// ModifiedFiles are the list of files that have been modified in this
		// pull request.
		ModifiedFiles []string
		// Comments are what our mock user writes to the pull request.
		Comments []string
		// ExpAutomerge is true if we expect Atlantis to automerge.
		ExpAutomerge bool
		// ExpAutoplan is true if we expect Atlantis to autoplan.
		ExpAutoplan bool
		// ExpParallel is true if we expect Atlantis to run parallel plans or applies.
		ExpParallel bool
		// ExpReplies is a list of files containing the expected replies that
		// Atlantis writes to the pull request in order. A reply from a parallel operation
		// will be matched using a substring check.
		ExpReplies [][]string
		// PolicyCheckEnabled runs integration tests through PolicyCheckProjectCommandBuilder.
		PolicyCheckEnabled bool
	}{
		{
			Description:   "simple",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply.txt"},
				{"exp-output-merge.txt"},
			},
			ExpAutoplan:        true,
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with plan comment",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-autoplan.txt"},
				{"exp-output-apply.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with comment -var",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=overridden",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-atlantis-plan-var-overridden.txt"},
				{"exp-output-apply-var.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with workspaces",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=default_workspace",
				"atlantis plan -w new_workspace -- -var var=new_workspace",
				"atlantis apply -w default",
				"atlantis apply -w new_workspace",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-atlantis-plan.txt"},
				{"exp-output-atlantis-plan-new-workspace.txt"},
				{"exp-output-apply-var-default-workspace.txt"},
				{"exp-output-apply-var-new-workspace.txt"},
				{"exp-output-merge-workspaces.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with workspaces and apply all",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=default_workspace",
				"atlantis plan -w new_workspace -- -var var=new_workspace",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-atlantis-plan.txt"},
				{"exp-output-atlantis-plan-new-workspace.txt"},
				{"exp-output-apply-var-all.txt"},
				{"exp-output-merge-workspaces.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with atlantis.yaml",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -w staging",
				"atlantis apply -w default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with atlantis.yaml and apply all",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-all.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "simple with atlantis.yaml and plan/apply all",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-all.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "modules staging only",
			RepoDir:       "modules",
			ModifiedFiles: []string{"staging/main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -d staging",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan-only-staging.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-merge-only-staging.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "modules modules only",
			RepoDir:       "modules",
			ModifiedFiles: []string{"modules/null/main.tf"},
			ExpAutoplan:   false,
			Comments: []string{
				"atlantis plan -d staging",
				"atlantis plan -d production",
				"atlantis apply -d staging",
				"atlantis apply -d production",
			},
			ExpReplies: [][]string{
				{"exp-output-plan-staging.txt"},
				{"exp-output-plan-production.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-production.txt"},
				{"exp-output-merge-all-dirs.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "modules-yaml",
			RepoDir:       "modules-yaml",
			ModifiedFiles: []string{"modules/null/main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -d staging",
				"atlantis apply -d production",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-production.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "tfvars-yaml",
			RepoDir:       "tfvars-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -p staging",
				"atlantis apply -p default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "tfvars no autoplan",
			RepoDir:       "tfvars-yaml-no-autoplan",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   false,
			Comments: []string{
				"atlantis plan -p staging",
				"atlantis plan -p default",
				"atlantis apply -p staging",
				"atlantis apply -p default",
			},
			ExpReplies: [][]string{
				{"exp-output-plan-staging.txt"},
				{"exp-output-plan-default.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "automerge",
			RepoDir:       "automerge",
			ExpAutomerge:  true,
			ExpAutoplan:   true,
			ModifiedFiles: []string{"dir1/main.tf", "dir2/main.tf"},
			Comments: []string{
				"atlantis apply -d dir1",
				"atlantis apply -d dir2",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-dir1.txt"},
				{"exp-output-apply-dir2.txt"},
				{"exp-output-automerge.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "server-side cfg",
			RepoDir:       "server-side-cfg",
			ExpAutomerge:  false,
			ExpAutoplan:   true,
			ModifiedFiles: []string{"main.tf"},
			Comments: []string{
				"atlantis apply -w staging",
				"atlantis apply -w default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-apply-staging-workspace.txt"},
				{"exp-output-apply-default-workspace.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
		{
			Description:   "workspaces parallel with atlantis.yaml",
			RepoDir:       "workspace-parallel-yaml",
			ModifiedFiles: []string{"production/main.tf", "staging/main.tf"},
			ExpAutoplan:   true,
			ExpParallel:   true,
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan-staging.txt", "exp-output-autoplan-production.txt"},
				{"exp-output-apply-all-staging.txt", "exp-output-apply-all-production.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: false,
		},
	}
	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			RegisterMockTestingT(t)

			ctrl, vcsClient, githubGetter, atlantisWorkspace := setupE2E(t, c.RepoDir, c.PolicyCheckEnabled)
			// Set the repo to be cloned through the testing backdoor.
			repoDir, headSHA, cleanup := initializeRepo(t, c.RepoDir)
			defer cleanup()
			atlantisWorkspace.TestingOverrideHeadCloneURL = fmt.Sprintf("file://%s", repoDir)

			// Setup test dependencies.
			w := httptest.NewRecorder()
			When(githubGetter.GetPullRequest(AnyRepo(), AnyInt())).ThenReturn(GitHubPullRequestParsed(headSHA), nil)
			When(vcsClient.GetModifiedFiles(AnyRepo(), matchers.AnyModelsPullRequest())).ThenReturn(c.ModifiedFiles, nil)

			// First, send the open pull request event which triggers autoplan.
			pullOpenedReq := GitHubPullRequestOpenedEvent(t, headSHA)
			ctrl.Post(w, pullOpenedReq)
			responseContains(t, w, 200, "Processing...")

			// Now send any other comments.
			for _, comment := range c.Comments {
				commentReq := GitHubCommentEvent(t, comment)
				w = httptest.NewRecorder()
				ctrl.Post(w, commentReq)
				responseContains(t, w, 200, "Processing...")
			}

			// Send the "pull closed" event which would be triggered by the
			// automerge or a manual merge.
			pullClosedReq := GitHubPullRequestClosedEvent(t)
			w = httptest.NewRecorder()
			ctrl.Post(w, pullClosedReq)
			responseContains(t, w, 200, "Pull request cleaned successfully")

			// Now we're ready to verify Atlantis made all the comments back (or
			// replies) that we expect.  We expect each plan to have 2 comments,
			// one for plan one for policy check and apply have 1 for each
			// comment plus one for the locks deleted at the end.
			expNumReplies := len(c.Comments) + 1

			if c.ExpAutoplan {
				expNumReplies++
			}

			// When enabled policy_check runs right after plan. So whenever
			// comment matches plan we add additional call to expected
			// number.
			if c.PolicyCheckEnabled {
				var planRegex = regexp.MustCompile("plan")
				for _, comment := range c.Comments {
					if planRegex.MatchString(comment) {
						expNumReplies++
					}
				}

				// Adding 1 for policy_check autorun
				if c.ExpAutoplan {
					expNumReplies++
				}
			}

			if c.ExpAutomerge {
				expNumReplies++
			}

			_, _, actReplies, _ := vcsClient.VerifyWasCalled(Times(expNumReplies)).CreateComment(AnyRepo(), AnyInt(), AnyString(), AnyString()).GetAllCapturedArguments()
			Assert(t, len(c.ExpReplies) == len(actReplies), "missing expected replies, got %d but expected %d", len(actReplies), len(c.ExpReplies))
			for i, expReply := range c.ExpReplies {
				assertCommentEquals(t, expReply, actReplies[i], c.RepoDir, c.ExpParallel)
			}

			if c.ExpAutomerge {
				// Verify that the merge API call was made.
				vcsClient.VerifyWasCalledOnce().MergePull(matchers.AnyModelsPullRequest())
			} else {
				vcsClient.VerifyWasCalled(Never()).MergePull(matchers.AnyModelsPullRequest())
			}
		})
	}
}

func TestGitHubWorkflowWithPolicyCheck(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	// Ensure we have >= TF 0.12 locally.
	ensureRunning012(t)

	cases := []struct {
		Description string
		// RepoDir is relative to testfixtures/test-repos.
		RepoDir string
		// ModifiedFiles are the list of files that have been modified in this
		// pull request.
		ModifiedFiles []string
		// Comments are what our mock user writes to the pull request.
		Comments []string
		// ExpAutomerge is true if we expect Atlantis to automerge.
		ExpAutomerge bool
		// ExpAutoplan is true if we expect Atlantis to autoplan.
		ExpAutoplan bool
		// ExpParallel is true if we expect Atlantis to run parallel plans or applies.
		ExpParallel bool
		// ExpReplies is a list of files containing the expected replies that
		// Atlantis writes to the pull request in order. A reply from a parallel operation
		// will be matched using a substring check.
		ExpReplies [][]string
		// PolicyCheckEnabled runs integration tests through PolicyCheckProjectCommandBuilder.
		PolicyCheckEnabled bool
	}{
		{
			Description:   "simple",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply.txt"},
				{"exp-output-merge.txt"},
			},
			ExpAutoplan:        true,
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with plan comment",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "policy check enabled: simple with plan comment",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with comment -var",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=overridden",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-atlantis-plan-var-overridden.txt"},
				{"exp-output-atlantis-policy-check-var-overriden.txt"},
				{"exp-output-apply-var.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with workspaces",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=default_workspace",
				"atlantis plan -w new_workspace -- -var var=new_workspace",
				"atlantis apply -w default",
				"atlantis apply -w new_workspace",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-atlantis-plan.txt"},
				{"exp-output-atlantis-policy-check.txt"},
				{"exp-output-atlantis-plan-new-workspace.txt"},
				{"exp-output-atlantis-policy-check-new-workspace.txt"},
				{"exp-output-apply-var-default-workspace.txt"},
				{"exp-output-apply-var-new-workspace.txt"},
				{"exp-output-merge-workspaces.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with workspaces and apply all",
			RepoDir:       "simple",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan -- -var var=default_workspace",
				"atlantis plan -w new_workspace -- -var var=new_workspace",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-atlantis-plan.txt"},
				{"exp-output-atlantis-policy-check.txt"},
				{"exp-output-atlantis-plan-new-workspace.txt"},
				{"exp-output-atlantis-policy-check-new-workspace.txt"},
				{"exp-output-apply-var-all.txt"},
				{"exp-output-merge-workspaces.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with atlantis.yaml",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -w staging",
				"atlantis apply -w default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with atlantis.yaml and apply all",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-all.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "simple with atlantis.yaml and plan/apply all",
			RepoDir:       "simple-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis plan",
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-all.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "modules staging only",
			RepoDir:       "modules",
			ModifiedFiles: []string{"staging/main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -d staging",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan-only-staging.txt"},
				{"exp-output-auto-policy-check-only-staging.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-merge-only-staging.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "modules modules only",
			RepoDir:       "modules",
			ModifiedFiles: []string{"modules/null/main.tf"},
			ExpAutoplan:   false,
			Comments: []string{
				"atlantis plan -d staging",
				"atlantis plan -d production",
				"atlantis apply -d staging",
				"atlantis apply -d production",
			},
			ExpReplies: [][]string{
				{"exp-output-plan-staging.txt"},
				{"exp-output-policy-check-staging.txt"},
				{"exp-output-plan-production.txt"},
				{"exp-output-policy-check-production.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-production.txt"},
				{"exp-output-merge-all-dirs.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "modules-yaml",
			RepoDir:       "modules-yaml",
			ModifiedFiles: []string{"modules/null/main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -d staging",
				"atlantis apply -d production",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-production.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "tfvars-yaml",
			RepoDir:       "tfvars-yaml",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   true,
			Comments: []string{
				"atlantis apply -p staging",
				"atlantis apply -p default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "tfvars no autoplan",
			RepoDir:       "tfvars-yaml-no-autoplan",
			ModifiedFiles: []string{"main.tf"},
			ExpAutoplan:   false,
			Comments: []string{
				"atlantis plan -p staging",
				"atlantis plan -p default",
				"atlantis apply -p staging",
				"atlantis apply -p default",
			},
			ExpReplies: [][]string{
				{"exp-output-plan-staging.txt"},
				{"exp-output-policy-check-staging.txt"},
				{"exp-output-plan-default.txt"},
				{"exp-output-policy-check-default.txt"},
				{"exp-output-apply-staging.txt"},
				{"exp-output-apply-default.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "automerge",
			RepoDir:       "automerge",
			ExpAutomerge:  true,
			ExpAutoplan:   true,
			ModifiedFiles: []string{"dir1/main.tf", "dir2/main.tf"},
			Comments: []string{
				"atlantis apply -d dir1",
				"atlantis apply -d dir2",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-dir1.txt"},
				{"exp-output-apply-dir2.txt"},
				{"exp-output-automerge.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "server-side cfg",
			RepoDir:       "server-side-cfg",
			ExpAutomerge:  false,
			ExpAutoplan:   true,
			ModifiedFiles: []string{"main.tf"},
			Comments: []string{
				"atlantis apply -w staging",
				"atlantis apply -w default",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan.txt"},
				{"exp-output-auto-policy-check.txt"},
				{"exp-output-apply-staging-workspace.txt"},
				{"exp-output-apply-default-workspace.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
		{
			Description:   "workspaces parallel with atlantis.yaml",
			RepoDir:       "workspace-parallel-yaml",
			ModifiedFiles: []string{"production/main.tf", "staging/main.tf"},
			ExpAutoplan:   true,
			ExpParallel:   true,
			Comments: []string{
				"atlantis apply",
			},
			ExpReplies: [][]string{
				{"exp-output-autoplan-staging.txt", "exp-output-autoplan-production.txt"},
				{"exp-output-auto-policy-check.txt", "exp-output-auto-policy-check.txt"},
				{"exp-output-apply-all-staging.txt", "exp-output-apply-all-production.txt"},
				{"exp-output-merge.txt"},
			},
			PolicyCheckEnabled: true,
		},
	}
	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			RegisterMockTestingT(t)

			ctrl, vcsClient, githubGetter, atlantisWorkspace := setupE2E(t, c.RepoDir, c.PolicyCheckEnabled)
			// Set the repo to be cloned through the testing backdoor.
			repoDir, headSHA, cleanup := initializeRepo(t, c.RepoDir)
			defer cleanup()
			atlantisWorkspace.TestingOverrideHeadCloneURL = fmt.Sprintf("file://%s", repoDir)

			// Setup test dependencies.
			w := httptest.NewRecorder()
			When(githubGetter.GetPullRequest(AnyRepo(), AnyInt())).ThenReturn(GitHubPullRequestParsed(headSHA), nil)
			When(vcsClient.GetModifiedFiles(AnyRepo(), matchers.AnyModelsPullRequest())).ThenReturn(c.ModifiedFiles, nil)

			// First, send the open pull request event which triggers autoplan.
			pullOpenedReq := GitHubPullRequestOpenedEvent(t, headSHA)
			ctrl.Post(w, pullOpenedReq)
			responseContains(t, w, 200, "Processing...")

			// Now send any other comments.
			for _, comment := range c.Comments {
				commentReq := GitHubCommentEvent(t, comment)
				w = httptest.NewRecorder()
				ctrl.Post(w, commentReq)
				responseContains(t, w, 200, "Processing...")
			}

			// Send the "pull closed" event which would be triggered by the
			// automerge or a manual merge.
			pullClosedReq := GitHubPullRequestClosedEvent(t)
			w = httptest.NewRecorder()
			ctrl.Post(w, pullClosedReq)
			responseContains(t, w, 200, "Pull request cleaned successfully")

			// Now we're ready to verify Atlantis made all the comments back (or
			// replies) that we expect.  We expect each plan to have 2 comments,
			// one for plan one for policy check and apply have 1 for each
			// comment plus one for the locks deleted at the end.
			expNumReplies := len(c.Comments) + 1

			if c.ExpAutoplan {
				expNumReplies++
			}

			// When enabled policy_check runs right after plan. So whenever
			// comment matches plan we add additional call to expected
			// number.
			if c.PolicyCheckEnabled {
				var planRegex = regexp.MustCompile("plan")
				for _, comment := range c.Comments {
					if planRegex.MatchString(comment) {
						expNumReplies++
					}
				}

				// Adding 1 for policy_check autorun
				if c.ExpAutoplan {
					expNumReplies++
				}
			}

			if c.ExpAutomerge {
				expNumReplies++
			}

			_, _, actReplies, _ := vcsClient.VerifyWasCalled(Times(expNumReplies)).CreateComment(AnyRepo(), AnyInt(), AnyString(), AnyString()).GetAllCapturedArguments()
			Assert(t, len(c.ExpReplies) == len(actReplies), "missing expected replies, got %d but expected %d", len(actReplies), len(c.ExpReplies))
			for i, expReply := range c.ExpReplies {
				assertCommentEquals(t, expReply, actReplies[i], c.RepoDir, c.ExpParallel)
			}

			if c.ExpAutomerge {
				// Verify that the merge API call was made.
				vcsClient.VerifyWasCalledOnce().MergePull(matchers.AnyModelsPullRequest())
			} else {
				vcsClient.VerifyWasCalled(Never()).MergePull(matchers.AnyModelsPullRequest())
			}
		})
	}
}

func setupE2E(t *testing.T, repoDir string, policyChecksEnabled bool) (server.EventsController, *vcsmocks.MockClient, *mocks.MockGithubPullGetter, *events.FileWorkspace) {
	allowForkPRs := false
	dataDir, binDir, cacheDir, cleanup := mkSubDirs(t)
	defer cleanup()

	//env vars

	if policyChecksEnabled {
		// need this to be set or we'll fail the policy check step
		os.Setenv(policy.DefaultConftestVersionEnvKey, "0.21.0")
	}

	// Mocks.
	e2eVCSClient := vcsmocks.NewMockClient()
	e2eStatusUpdater := &events.DefaultCommitStatusUpdater{Client: e2eVCSClient}
	e2eGithubGetter := mocks.NewMockGithubPullGetter()
	e2eGitlabGetter := mocks.NewMockGitlabMergeRequestGetter()

	// Real dependencies.
	logger := logging.NewSimpleLogger("server", true, logging.Debug)
	eventParser := &events.EventParser{
		GithubUser:  "github-user",
		GithubToken: "github-token",
		GitlabUser:  "gitlab-user",
		GitlabToken: "gitlab-token",
	}
	commentParser := &events.CommentParser{
		GithubUser: "github-user",
		GitlabUser: "gitlab-user",
	}
	terraformClient, err := terraform.NewClient(logger, binDir, cacheDir, "", "", "", "default-tf-version", "https://releases.hashicorp.com", &NoopTFDownloader{}, false)
	Ok(t, err)
	boltdb, err := db.New(dataDir)
	Ok(t, err)
	lockingClient := locking.NewClient(boltdb)
	projectLocker := &events.DefaultProjectLocker{
		Locker:    lockingClient,
		VCSClient: e2eVCSClient,
	}
	workingDir := &events.FileWorkspace{
		DataDir:                     dataDir,
		TestingOverrideHeadCloneURL: "override-me",
	}

	defaultTFVersion := terraformClient.DefaultVersion()
	locker := events.NewDefaultWorkingDirLocker()
	parser := &yaml.ParserValidator{}
	globalCfg := valid.NewGlobalCfg(true, false, false)
	expCfgPath := filepath.Join(absRepoPath(t, repoDir), "repos.yaml")
	if _, err := os.Stat(expCfgPath); err == nil {
		globalCfg, err = parser.ParseGlobalCfg(expCfgPath, globalCfg)
		Ok(t, err)
	}
	drainer := &events.Drainer{}
	preWorkflowHooksCommandRunner := &events.DefaultPreWorkflowHooksCommandRunner{
		VCSClient:             e2eVCSClient,
		GlobalCfg:             globalCfg,
		Logger:                logger,
		WorkingDirLocker:      locker,
		WorkingDir:            workingDir,
		Drainer:               drainer,
		PreWorkflowHookRunner: &runtime.PreWorkflowHookRunner{},
	}
	statsScope := stats.NewStore(stats.NewNullSink(), false)

	projectCommandBuilder := events.NewProjectCommandBuilder(
		policyChecksEnabled,
		parser,
		&events.DefaultProjectFinder{},
		e2eVCSClient,
		workingDir,
		locker,
		globalCfg,
		&events.DefaultPendingPlanFinder{},
		commentParser,
		false,
		statsScope,
		logger,
	)

	showStepRunner, err := runtime.NewShowStepRunner(terraformClient, defaultTFVersion)

	Ok(t, err)

	policyCheckRunner, err := runtime.NewPolicyCheckStepRunner(
		defaultTFVersion,
		policy.NewConfTestExecutorWorkflow(logger, binDir, &NoopTFDownloader{}),
	)

	Ok(t, err)

	commandRunner := &events.DefaultCommandRunner{
		ProjectCommandRunner: &events.DefaultProjectCommandRunner{
			Locker:           projectLocker,
			LockURLGenerator: &mockLockURLGenerator{},
			InitStepRunner: &runtime.InitStepRunner{
				TerraformExecutor: terraformClient,
				DefaultTFVersion:  defaultTFVersion,
			},
			PlanStepRunner: &runtime.PlanStepRunner{
				TerraformExecutor: terraformClient,
				DefaultTFVersion:  defaultTFVersion,
			},
			ShowStepRunner:        showStepRunner,
			PolicyCheckStepRunner: policyCheckRunner,
			ApplyStepRunner: &runtime.ApplyStepRunner{
				TerraformExecutor: terraformClient,
			},
			RunStepRunner: &runtime.RunStepRunner{
				TerraformExecutor: terraformClient,
				DefaultTFVersion:  defaultTFVersion,
			},
			PullApprovedChecker: e2eVCSClient,
			WorkingDir:          workingDir,
			Webhooks:            &mockWebhookSender{},
			WorkingDirLocker:    locker,
		},
		EventParser:              eventParser,
		VCSClient:                e2eVCSClient,
		GithubPullGetter:         e2eGithubGetter,
		GitlabMergeRequestGetter: e2eGitlabGetter,
		CommitStatusUpdater:      e2eStatusUpdater,
		MarkdownRenderer:         &events.MarkdownRenderer{},
		Logger:                   logger,
		StatsScope:               statsScope,
		AllowForkPRs:             allowForkPRs,
		AllowForkPRsFlag:         "allow-fork-prs",
		ProjectCommandBuilder:    projectCommandBuilder,
		DB:                       boltdb,
		PendingPlanFinder:        &events.DefaultPendingPlanFinder{},
		GlobalAutomerge:          false,
		WorkingDir:               workingDir,
		Drainer:                  drainer,
	}

	repoAllowlistChecker, err := events.NewRepoAllowlistChecker("*")
	Ok(t, err)

	ctrl := server.EventsController{
		TestingMode:                   true,
		PreWorkflowHooksCommandRunner: preWorkflowHooksCommandRunner,
		CommandRunner:                 commandRunner,
		PullCleaner: &events.PullClosedExecutor{
			Locker:     lockingClient,
			VCSClient:  e2eVCSClient,
			WorkingDir: workingDir,
			DB:         boltdb,
		},
		Logger:                       logger,
		Parser:                       eventParser,
		CommentParser:                commentParser,
		GithubWebhookSecret:          nil,
		GithubRequestValidator:       &server.DefaultGithubRequestValidator{},
		GitlabRequestParserValidator: &server.DefaultGitlabRequestParserValidator{},
		GitlabWebhookSecret:          nil,
		RepoAllowlistChecker:         repoAllowlistChecker,
		SupportedVCSHosts:            []models.VCSHostType{models.Gitlab, models.Github, models.BitbucketCloud},
		VCSClient:                    e2eVCSClient,
	}
	return ctrl, e2eVCSClient, e2eGithubGetter, workingDir
}

type mockLockURLGenerator struct{}

func (m *mockLockURLGenerator) GenerateLockURL(lockID string) string {
	return "lock-url"
}

type mockWebhookSender struct{}

func (w *mockWebhookSender) Send(log *logging.SimpleLogger, result webhooks.ApplyResult) error {
	return nil
}

func GitHubCommentEvent(t *testing.T, comment string) *http.Request {
	requestJSON, err := ioutil.ReadFile(filepath.Join("testfixtures", "githubIssueCommentEvent.json"))
	Ok(t, err)
	requestJSON = []byte(strings.Replace(string(requestJSON), "###comment body###", comment, 1))
	req, err := http.NewRequest("POST", "/events", bytes.NewBuffer(requestJSON))
	Ok(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "issue_comment")
	return req
}

func GitHubPullRequestOpenedEvent(t *testing.T, headSHA string) *http.Request {
	requestJSON, err := ioutil.ReadFile(filepath.Join("testfixtures", "githubPullRequestOpenedEvent.json"))
	Ok(t, err)
	// Replace sha with expected sha.
	requestJSONStr := strings.Replace(string(requestJSON), "c31fd9ea6f557ad2ea659944c3844a059b83bc5d", headSHA, -1)
	req, err := http.NewRequest("POST", "/events", bytes.NewBuffer([]byte(requestJSONStr)))
	Ok(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "pull_request")
	return req
}

func GitHubPullRequestClosedEvent(t *testing.T) *http.Request {
	requestJSON, err := ioutil.ReadFile(filepath.Join("testfixtures", "githubPullRequestClosedEvent.json"))
	Ok(t, err)
	req, err := http.NewRequest("POST", "/events", bytes.NewBuffer(requestJSON))
	Ok(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "pull_request")
	return req
}

func GitHubPullRequestParsed(headSHA string) *github.PullRequest {
	// headSHA can't be empty so default if not set.
	if headSHA == "" {
		headSHA = "13940d121be73f656e2132c6d7b4c8e87878ac8d"
	}
	return &github.PullRequest{
		Number:  github.Int(2),
		State:   github.String("open"),
		HTMLURL: github.String("htmlurl"),
		Head: &github.PullRequestBranch{
			Repo: &github.Repository{
				FullName: github.String("runatlantis/atlantis-tests"),
				CloneURL: github.String("https://github.com/runatlantis/atlantis-tests.git"),
			},
			SHA: github.String(headSHA),
			Ref: github.String("branch"),
		},
		Base: &github.PullRequestBranch{
			Repo: &github.Repository{
				FullName: github.String("runatlantis/atlantis-tests"),
				CloneURL: github.String("https://github.com/runatlantis/atlantis-tests.git"),
			},
			Ref: github.String("master"),
		},
		User: &github.User{
			Login: github.String("atlantisbot"),
		},
	}
}

// absRepoPath returns the absolute path to the test repo under dir repoDir.
func absRepoPath(t *testing.T, repoDir string) string {
	path, err := filepath.Abs(filepath.Join("testfixtures", "test-repos", repoDir))
	Ok(t, err)
	return path
}

// initializeRepo copies the repo data from testfixtures and initializes a new
// git repo in a temp directory. It returns that directory and a function
// to run in a defer that will delete the dir.
// The purpose of this function is to create a real git repository with a branch
// called 'branch' from the files under repoDir. This is so we can check in
// those files normally to this repo without needing a .git directory.
func initializeRepo(t *testing.T, repoDir string) (string, string, func()) {
	originRepo := absRepoPath(t, repoDir)

	// Copy the files to the temp dir.
	destDir, cleanup := TempDir(t)
	runCmd(t, "", "cp", "-r", fmt.Sprintf("%s/.", originRepo), destDir)

	// Initialize the git repo.
	runCmd(t, destDir, "git", "init")
	runCmd(t, destDir, "touch", ".gitkeep")
	runCmd(t, destDir, "git", "add", ".gitkeep")
	runCmd(t, destDir, "git", "config", "--local", "user.email", "atlantisbot@runatlantis.io")
	runCmd(t, destDir, "git", "config", "--local", "user.name", "atlantisbot")
	runCmd(t, destDir, "git", "commit", "-m", "initial commit")
	runCmd(t, destDir, "git", "checkout", "-b", "branch")
	runCmd(t, destDir, "git", "add", ".")
	runCmd(t, destDir, "git", "commit", "-am", "branch commit")
	headSHA := runCmd(t, destDir, "git", "rev-parse", "HEAD")
	headSHA = strings.Trim(headSHA, "\n")

	return destDir, headSHA, cleanup
}

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	Assert(t, err == nil, "err running %q: %s", strings.Join(append([]string{name}, args...), " "), cpOut)
	return string(cpOut)
}

func assertCommentEquals(t *testing.T, expReplies []string, act string, repoDir string, parallel bool) {
	t.Helper()

	// Replace all 'Creation complete after 0s [id=2135833172528078362]' strings with
	// 'Creation complete after *s [id=*******************]' so we can do a comparison.
	idRegex := regexp.MustCompile(`Creation complete after [0-9]+s \[id=[0-9]+]`)
	act = idRegex.ReplaceAllString(act, "Creation complete after *s [id=*******************]")

	// Replace all null_resource.simple{n}: .* with null_resource.simple: because
	// with multiple resources being created the logs are all out of order which
	// makes comparison impossible.
	resourceRegex := regexp.MustCompile(`null_resource\.simple(\[\d])?\d?:.*`)
	act = resourceRegex.ReplaceAllString(act, "null_resource.simple:")

	// For parallel plans and applies, do a substring match since output may be out of order
	var replyMatchesExpected func(string, string) bool
	if parallel {
		replyMatchesExpected = func(act string, expStr string) bool {
			return strings.Contains(act, expStr)
		}
	} else {
		replyMatchesExpected = func(act string, expStr string) bool {
			return expStr == act
		}
	}

	for _, expFile := range expReplies {
		exp, err := ioutil.ReadFile(filepath.Join(absRepoPath(t, repoDir), expFile))
		Ok(t, err)
		expStr := string(exp)
		// My editor adds a newline to all the files, so if the actual comment
		// doesn't end with a newline then strip the last newline from the file's
		// contents.
		if !strings.HasSuffix(act, "\n") {
			expStr = strings.TrimSuffix(expStr, "\n")
		}

		if !replyMatchesExpected(act, expStr) {
			// If in CI, we write the diff to the console. Otherwise we write the diff
			// to file so we can use our local diff viewer.
			if os.Getenv("CI") == "true" {
				t.Logf("exp: %s, got: %s", expStr, act)
				t.FailNow()
			} else {
				actFile := filepath.Join(absRepoPath(t, repoDir), expFile+".act")
				err := ioutil.WriteFile(actFile, []byte(act), 0600)
				Ok(t, err)
				cwd, err := os.Getwd()
				Ok(t, err)
				rel, err := filepath.Rel(cwd, actFile)
				Ok(t, err)
				t.Errorf("%q was different, wrote actual comment to %q", expFile, rel)
			}
		}
	}
}

// returns parent, bindir, cachedir, cleanup func
func mkSubDirs(t *testing.T) (string, string, string, func()) {
	tmp, cleanup := TempDir(t)
	binDir := filepath.Join(tmp, "bin")
	err := os.MkdirAll(binDir, 0700)
	Ok(t, err)

	cachedir := filepath.Join(tmp, "plugin-cache")
	err = os.MkdirAll(cachedir, 0700)
	Ok(t, err)

	return tmp, binDir, cachedir, cleanup
}

// Will fail test if terraform isn't in path and isn't version >= 0.12
func ensureRunning012(t *testing.T) {
	localPath, err := exec.LookPath("terraform")
	if err != nil {
		t.Log("terraform >= 0.12 must be installed to run this test")
		t.FailNow()
	}
	versionOutBytes, err := exec.Command(localPath, "version").Output() // #nosec
	if err != nil {
		t.Logf("error running terraform version: %s", err)
		t.FailNow()
	}
	versionOutput := string(versionOutBytes)
	match := versionRegex.FindStringSubmatch(versionOutput)
	if len(match) <= 1 {
		t.Logf("could not parse terraform version from %s", versionOutput)
		t.FailNow()
	}
	localVersion, err := version.NewVersion(match[1])
	Ok(t, err)
	minVersion, err := version.NewVersion("0.12.0")
	Ok(t, err)
	if localVersion.LessThan(minVersion) {
		t.Logf("must have terraform version >= %s, you have %s", minVersion, localVersion)
		t.FailNow()
	}
}

// versionRegex extracts the version from `terraform version` output.
//     Terraform v0.12.0-alpha4 (2c36829d3265661d8edbd5014de8090ea7e2a076)
//	   => 0.12.0-alpha4
//
//     Terraform v0.11.10
//	   => 0.11.10
var versionRegex = regexp.MustCompile("Terraform v(.*?)(\\s.*)?\n")
