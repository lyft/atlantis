// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.

package events_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/legacy/events"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/models"
	. "github.com/runatlantis/atlantis/testing"
)

var parser = events.EventParser{
	AllowDraftPRs:      false,
	BitbucketUser:      "bitbucket-user",
	BitbucketToken:     "bitbucket-token",
	BitbucketServerURL: "http://mycorp.com:7490",
}

func TestNewCommand_CleansDir(t *testing.T) {
	cases := []struct {
		RepoRelDir string
		ExpDir     string
	}{
		{
			"",
			"",
		},
		{
			"/",
			".",
		},
		{
			"./",
			".",
		},
		// We rely on our callers to not pass in relative dirs.
		{
			"..",
			"..",
		},
	}

	for _, c := range cases {
		t.Run(c.RepoRelDir, func(t *testing.T) {
			cmd := events.NewCommentCommand(c.RepoRelDir, nil, command.Plan, false, "workspace", "")
			Equals(t, c.ExpDir, cmd.RepoRelDir)
		})
	}
}

func TestNewCommand_EmptyDirWorkspaceProject(t *testing.T) {
	cmd := events.NewCommentCommand("", nil, command.Plan, false, "", "")
	Equals(t, command.Comment{
		RepoRelDir:  "",
		Flags:       nil,
		Name:        command.Plan,
		Workspace:   "",
		ProjectName: "",
	}, *cmd)
}

func TestNewCommand_AllFieldsSet(t *testing.T) {
	cmd := events.NewCommentCommand("dir", []string{"a", "b"}, command.Plan, true, "workspace", "project")
	Equals(t, command.Comment{
		Workspace:   "workspace",
		RepoRelDir:  "dir",
		Flags:       []string{"a", "b"},
		ForceApply:  true,
		Name:        command.Plan,
		ProjectName: "project",
	}, *cmd)
}

func TestAutoplanCommand_CommandName(t *testing.T) {
	Equals(t, command.Plan, (events.AutoplanCommand{}).CommandName())
}

func TestAutoplanCommand_IsAutoplan(t *testing.T) {
	Equals(t, true, (events.AutoplanCommand{}).IsAutoplan())
}

func TestCommentCommand_CommandName(t *testing.T) {
	Equals(t, command.Plan, (command.Comment{
		Name: command.Plan,
	}).CommandName())
	Equals(t, command.Apply, (command.Comment{
		Name: command.Apply,
	}).CommandName())
}

func TestCommentCommand_IsAutoplan(t *testing.T) {
	Equals(t, false, (command.Comment{}).IsAutoplan())
}

func TestCommentCommand_String(t *testing.T) {
	exp := `command="plan" dir="mydir" workspace="myworkspace" project="myproject" loglevel="trace" flags="flag1,flag2"`
	Equals(t, exp, (command.Comment{
		RepoRelDir:  "mydir",
		Flags:       []string{"flag1", "flag2"},
		Name:        command.Plan,
		Workspace:   "myworkspace",
		ProjectName: "myproject",
		LogLevel:    "trace",
	}).String())
}

func TestParseBitbucketCloudCommentEvent_EmptyString(t *testing.T) {
	_, _, _, _, _, err := parser.ParseBitbucketCloudPullCommentEvent([]byte(""))
	ErrEquals(t, "parsing json: unexpected end of JSON input", err)
}

func TestParseBitbucketCloudCommentEvent_EmptyObject(t *testing.T) {
	_, _, _, _, _, err := parser.ParseBitbucketCloudPullCommentEvent([]byte("{}"))
	ErrContains(t, "Key: 'CommentEvent.CommonEventData.Actor' Error:Field validation for 'Actor' failed on the 'required' tag\nKey: 'CommentEvent.CommonEventData.Repository' Error:Field validation for 'Repository' failed on the 'required' tag\nKey: 'CommentEvent.CommonEventData.PullRequest' Error:Field validation for 'PullRequest' failed on the 'required' tag\nKey: 'CommentEvent.Comment' Error:Field validation for 'Comment' failed on the 'required' tag", err)
}

func TestParseBitbucketCloudCommentEvent_CommitHashMissing(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-cloud-comment-event.json")
	bytes, err := os.ReadFile(path)
	Ok(t, err)
	emptyCommitHash := strings.Replace(string(bytes), `        "hash": "e0624da46d3a",`, "", -1)
	_, _, _, _, _, err = parser.ParseBitbucketCloudPullCommentEvent([]byte(emptyCommitHash))
	ErrContains(t, "Key: 'CommentEvent.CommonEventData.PullRequest.Source.Commit.Hash' Error:Field validation for 'Hash' failed on the 'required' tag", err)
}

func TestParseBitbucketCloudCommentEvent_ValidEvent(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-cloud-comment-event.json")
	bytes, err := os.ReadFile(path)
	Ok(t, err)
	pull, baseRepo, _, user, comment, err := parser.ParseBitbucketCloudPullCommentEvent(bytes)
	Ok(t, err)
	expBaseRepo := models.Repo{
		FullName:          "lkysow/atlantis-example",
		Owner:             "lkysow",
		Name:              "atlantis-example",
		CloneURL:          "https://bitbucket-user:bitbucket-token@bitbucket.org/lkysow/atlantis-example.git",
		SanitizedCloneURL: "https://bitbucket-user:<redacted>@bitbucket.org/lkysow/atlantis-example.git",
		VCSHost: models.VCSHost{
			Hostname: "bitbucket.org",
			Type:     models.BitbucketCloud,
		},
	}
	Equals(t, expBaseRepo, baseRepo)
	Equals(t, models.PullRequest{
		Num:        2,
		HeadCommit: "e0624da46d3a",
		URL:        "https://bitbucket.org/lkysow/atlantis-example/pull-requests/2",
		HeadBranch: "lkysow/maintf-edited-online-with-bitbucket-1532029690581",
		BaseBranch: "master",
		Author:     "lkysow",
		State:      models.ClosedPullState,
		BaseRepo:   expBaseRepo,
		HeadRepo: models.Repo{
			FullName:          "lkysow-fork/atlantis-example",
			Owner:             "lkysow-fork",
			Name:              "atlantis-example",
			CloneURL:          "https://bitbucket-user:bitbucket-token@bitbucket.org/lkysow-fork/atlantis-example.git",
			SanitizedCloneURL: "https://bitbucket-user:<redacted>@bitbucket.org/lkysow-fork/atlantis-example.git",
			VCSHost: models.VCSHost{
				Hostname: "bitbucket.org",
				Type:     models.BitbucketCloud,
			},
		},
	}, pull)
	Equals(t, models.User{
		Username: "lkysow",
	}, user)
	Equals(t, "my comment", comment)
}

func TestParseBitbucketCloudCommentEvent_MultipleStates(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-cloud-comment-event.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}

	cases := []struct {
		pullState string
		exp       models.PullRequestState
	}{
		{
			"OPEN",
			models.OpenPullState,
		},
		{
			"MERGED",
			models.ClosedPullState,
		},
		{
			"SUPERSEDED",
			models.ClosedPullState,
		},
		{
			"DECLINED",
			models.ClosedPullState,
		},
	}

	for _, c := range cases {
		t.Run(c.pullState, func(t *testing.T) {
			withState := strings.Replace(string(bytes), `"state": "MERGED"`, fmt.Sprintf(`"state": "%s"`, c.pullState), -1)
			pull, _, _, _, _, err := parser.ParseBitbucketCloudPullCommentEvent([]byte(withState))
			Ok(t, err)
			Equals(t, c.exp, pull.State)
		})
	}
}

func TestParseBitbucketCloudPullEvent_ValidEvent(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-cloud-pull-event-created.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}
	pull, baseRepo, _, user, err := parser.ParseBitbucketCloudPullEvent(bytes)
	Ok(t, err)
	expBaseRepo := models.Repo{
		FullName:          "lkysow/atlantis-example",
		Owner:             "lkysow",
		Name:              "atlantis-example",
		CloneURL:          "https://bitbucket-user:bitbucket-token@bitbucket.org/lkysow/atlantis-example.git",
		SanitizedCloneURL: "https://bitbucket-user:<redacted>@bitbucket.org/lkysow/atlantis-example.git",
		VCSHost: models.VCSHost{
			Hostname: "bitbucket.org",
			Type:     models.BitbucketCloud,
		},
	}
	Equals(t, expBaseRepo, baseRepo)
	Equals(t, models.PullRequest{
		Num:        16,
		HeadCommit: "1e69a602caef",
		URL:        "https://bitbucket.org/lkysow/atlantis-example/pull-requests/16",
		HeadBranch: "Luke/maintf-edited-online-with-bitbucket-1560433073473",
		BaseBranch: "master",
		Author:     "Luke",
		State:      models.OpenPullState,
		BaseRepo:   expBaseRepo,
		HeadRepo: models.Repo{
			FullName:          "lkysow-fork/atlantis-example",
			Owner:             "lkysow-fork",
			Name:              "atlantis-example",
			CloneURL:          "https://bitbucket-user:bitbucket-token@bitbucket.org/lkysow-fork/atlantis-example.git",
			SanitizedCloneURL: "https://bitbucket-user:<redacted>@bitbucket.org/lkysow-fork/atlantis-example.git",
			VCSHost: models.VCSHost{
				Hostname: "bitbucket.org",
				Type:     models.BitbucketCloud,
			},
		},
	}, pull)
	Equals(t, models.User{
		Username: "Luke",
	}, user)
}

func TestParseBitbucketCloudPullEvent_States(t *testing.T) {
	for _, c := range []struct {
		JSON     string
		ExpState models.PullRequestState
	}{
		{
			JSON:     "bitbucket-cloud-pull-event-created.json",
			ExpState: models.OpenPullState,
		},
		{
			JSON:     "bitbucket-cloud-pull-event-fulfilled.json",
			ExpState: models.ClosedPullState,
		},
		{
			JSON:     "bitbucket-cloud-pull-event-rejected.json",
			ExpState: models.ClosedPullState,
		},
	} {
		path := filepath.Join("testdata", c.JSON)
		bytes, err := os.ReadFile(path)
		if err != nil {
			Ok(t, err)
		}
		pull, _, _, _, err := parser.ParseBitbucketCloudPullEvent(bytes)
		Ok(t, err)
		Equals(t, c.ExpState, pull.State)
	}
}

func TestGetBitbucketCloudEventType(t *testing.T) {
	cases := []struct {
		header string
		exp    models.PullRequestEventType
	}{
		{
			header: "pullrequest:created",
			exp:    models.OpenedPullEvent,
		},
		{
			header: "pullrequest:updated",
			exp:    models.UpdatedPullEvent,
		},
		{
			header: "pullrequest:fulfilled",
			exp:    models.ClosedPullEvent,
		},
		{
			header: "pullrequest:rejected",
			exp:    models.ClosedPullEvent,
		},
		{
			header: "random",
			exp:    models.OtherPullEvent,
		},
	}
	for _, c := range cases {
		t.Run(c.header, func(t *testing.T) {
			act := parser.GetBitbucketCloudPullEventType(c.header)
			Equals(t, c.exp, act)
		})
	}
}

func TestParseBitbucketServerCommentEvent_EmptyString(t *testing.T) {
	_, _, _, _, _, err := parser.ParseBitbucketServerPullCommentEvent([]byte(""))
	ErrEquals(t, "parsing json: unexpected end of JSON input", err)
}

func TestParseBitbucketServerCommentEvent_EmptyObject(t *testing.T) {
	_, _, _, _, _, err := parser.ParseBitbucketServerPullCommentEvent([]byte("{}"))
	ErrContains(t, `API response "{}" was missing fields: Key: 'CommentEvent.CommonEventData.Actor' Error:Field validation for 'Actor' failed on the 'required' tag`, err)
}

func TestParseBitbucketServerCommentEvent_CommitHashMissing(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-server-comment-event.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}
	emptyCommitHash := strings.Replace(string(bytes), `"latestCommit": "bfb1af1ba9c2a2fa84cd61af67e6e1b60a22e060",`, "", -1)
	_, _, _, _, _, err = parser.ParseBitbucketServerPullCommentEvent([]byte(emptyCommitHash))
	ErrContains(t, "Key: 'CommentEvent.CommonEventData.PullRequest.FromRef.LatestCommit' Error:Field validation for 'LatestCommit' failed on the 'required' tag", err)
}

func TestParseBitbucketServerCommentEvent_ValidEvent(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-server-comment-event.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}
	pull, baseRepo, _, user, comment, err := parser.ParseBitbucketServerPullCommentEvent(bytes)
	Ok(t, err)
	expBaseRepo := models.Repo{
		FullName:          "atlantis/atlantis-example",
		Owner:             "atlantis",
		Name:              "atlantis-example",
		CloneURL:          "http://bitbucket-user:bitbucket-token@mycorp.com:7490/scm/at/atlantis-example.git",
		SanitizedCloneURL: "http://bitbucket-user:<redacted>@mycorp.com:7490/scm/at/atlantis-example.git",
		VCSHost: models.VCSHost{
			Hostname: "mycorp.com",
			Type:     models.BitbucketServer,
		},
	}
	Equals(t, expBaseRepo, baseRepo)
	Equals(t, models.PullRequest{
		Num:        1,
		HeadCommit: "bfb1af1ba9c2a2fa84cd61af67e6e1b60a22e060",
		URL:        "http://mycorp.com:7490/projects/AT/repos/atlantis-example/pull-requests/1",
		HeadBranch: "branch",
		BaseBranch: "master",
		Author:     "lkysow",
		State:      models.OpenPullState,
		BaseRepo:   expBaseRepo,
		HeadRepo: models.Repo{
			FullName:          "atlantis-fork/atlantis-example",
			Owner:             "atlantis-fork",
			Name:              "atlantis-example",
			CloneURL:          "http://bitbucket-user:bitbucket-token@mycorp.com:7490/scm/fk/atlantis-example.git",
			SanitizedCloneURL: "http://bitbucket-user:<redacted>@mycorp.com:7490/scm/fk/atlantis-example.git",
			VCSHost: models.VCSHost{
				Hostname: "mycorp.com",
				Type:     models.BitbucketServer,
			},
		},
	}, pull)
	Equals(t, models.User{
		Username: "lkysow",
	}, user)
	Equals(t, "atlantis plan", comment)
}

func TestParseBitbucketServerCommentEvent_MultipleStates(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-server-comment-event.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}

	cases := []struct {
		pullState string
		exp       models.PullRequestState
	}{
		{
			"OPEN",
			models.OpenPullState,
		},
		{
			"MERGED",
			models.ClosedPullState,
		},
		{
			"DECLINED",
			models.ClosedPullState,
		},
	}

	for _, c := range cases {
		t.Run(c.pullState, func(t *testing.T) {
			withState := strings.Replace(string(bytes), `"state": "OPEN"`, fmt.Sprintf(`"state": "%s"`, c.pullState), -1)
			pull, _, _, _, _, err := parser.ParseBitbucketServerPullCommentEvent([]byte(withState))
			Ok(t, err)
			Equals(t, c.exp, pull.State)
		})
	}
}

func TestParseBitbucketServerPullEvent_ValidEvent(t *testing.T) {
	path := filepath.Join("testdata", "bitbucket-server-pull-event-merged.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		Ok(t, err)
	}
	pull, baseRepo, _, user, err := parser.ParseBitbucketServerPullEvent(bytes)
	Ok(t, err)
	expBaseRepo := models.Repo{
		FullName:          "atlantis/atlantis-example",
		Owner:             "atlantis",
		Name:              "atlantis-example",
		CloneURL:          "http://bitbucket-user:bitbucket-token@mycorp.com:7490/scm/at/atlantis-example.git",
		SanitizedCloneURL: "http://bitbucket-user:<redacted>@mycorp.com:7490/scm/at/atlantis-example.git",
		VCSHost: models.VCSHost{
			Hostname: "mycorp.com",
			Type:     models.BitbucketServer,
		},
	}
	Equals(t, expBaseRepo, baseRepo)
	Equals(t, models.PullRequest{
		Num:        2,
		HeadCommit: "86a574157f5a2dadaf595b9f06c70fdfdd039912",
		URL:        "http://mycorp.com:7490/projects/AT/repos/atlantis-example/pull-requests/2",
		HeadBranch: "branch",
		BaseBranch: "master",
		Author:     "lkysow",
		State:      models.ClosedPullState,
		BaseRepo:   expBaseRepo,
		HeadRepo: models.Repo{
			FullName:          "atlantis-fork/atlantis-example",
			Owner:             "atlantis-fork",
			Name:              "atlantis-example",
			CloneURL:          "http://bitbucket-user:bitbucket-token@mycorp.com:7490/scm/fk/atlantis-example.git",
			SanitizedCloneURL: "http://bitbucket-user:<redacted>@mycorp.com:7490/scm/fk/atlantis-example.git",
			VCSHost: models.VCSHost{
				Hostname: "mycorp.com",
				Type:     models.BitbucketServer,
			},
		},
	}, pull)
	Equals(t, models.User{
		Username: "lkysow",
	}, user)
}

func TestGetBitbucketServerEventType(t *testing.T) {
	cases := []struct {
		header string
		exp    models.PullRequestEventType
	}{
		{
			header: "pr:opened",
			exp:    models.OpenedPullEvent,
		},
		{
			header: "pr:merged",
			exp:    models.ClosedPullEvent,
		},
		{
			header: "pr:declined",
			exp:    models.ClosedPullEvent,
		},
		{
			header: "pr:deleted",
			exp:    models.ClosedPullEvent,
		},
		{
			header: "random",
			exp:    models.OtherPullEvent,
		},
	}
	for _, c := range cases {
		t.Run(c.header, func(t *testing.T) {
			act := parser.GetBitbucketServerPullEventType(c.header)
			Equals(t, c.exp, act)
		})
	}
}
