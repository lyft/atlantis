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
	"testing"

	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/command"
	. "github.com/runatlantis/atlantis/testing"
)

var parser = events.EventParser{
	AllowDraftPRs:      false,
	BitbucketUser:      "bitbucket-user",
	BitbucketToken:     "bitbucket-token",
	BitbucketServerURL: "http://mycorp.com:7490",
	AzureDevopsUser:    "azuredevops-user",
	AzureDevopsToken:   "azuredevops-token",
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
