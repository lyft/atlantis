package events

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	version "github.com/hashicorp/go-version"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/config"
	"github.com/runatlantis/atlantis/server/config/valid"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/legacy/events/matchers"
	vcsmocks "github.com/runatlantis/atlantis/server/legacy/events/vcs/mocks"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	. "github.com/runatlantis/atlantis/testing"
)

// Test different permutations of global and repo config.
func TestBuildProjectCmdCtx(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	emptyPolicySets := valid.PolicySets{
		Version:    nil,
		PolicySets: []valid.PolicySet{},
	}
	baseRepo := models.Repo{
		FullName: "owner/repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
		},
	}
	pull := models.PullRequest{
		BaseRepo: baseRepo,
	}
	cases := map[string]struct {
		globalCfg    string
		repoCfg      string
		expErr       string
		expCtx       command.ProjectContext
		expPlanSteps []string
	}{
		// Test that if we've set global defaults and no project config
		// that the global defaults are used.
		"global defaults": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
pull_request_workflows:
  default:
    plan:
      steps:
      - init
      - plan`,
			repoCfg: "",
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{},
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"init", "plan"},
		},

		// Test that if we've set global defaults, that they are used but the
		// allowed project config values also come through.
		"global defaults with repo cfg": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
pull_request_workflows:
  default:
    plan:
      steps:
      - init
      - plan`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
  `,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"init", "plan"},
		},

		// Set a global apply req that should be used.
		"global apply_requirements": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
  apply_requirements: [approved, mergeable]
pull_request_workflows:
  default:
    plan:
      steps:
      - init
      - plan`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
`,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{"approved", "mergeable"},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"init", "plan"},
		},

		// If we have global config that matches a specific repo, it should be used.
		"specific repo": {
			globalCfg: `
repos:
- id: /.*/
  pull_request_workflow: default
- id: github.com/owner/repo
  pull_request_workflow: specific
  apply_requirements: [approved]
pull_request_workflows:
  default:
    plan:
      steps:
      - init
      - plan
  specific:
    plan:
      steps:
      - plan`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
`,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{"approved"},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"plan"},
		},

		// We should get an error if the repo sets an apply req when its
		// not allowed.
		"repo defines apply_requirements": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
  apply_requirements: [approved, mergeable]
pull_request_workflows:
  default:
    plan:
      steps:
        - init
        - plan`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  apply_requirements: []
`,
			expErr: "repo config not allowed to set 'apply_requirements' key: server-side config needs 'allowed_overrides: [apply_requirements]'",
		},

		// We should get an error if a repo sets a workflow when it's not allowed.
		"repo sets its own workflow": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
  apply_requirements: [approved, mergeable]
pull_request_workflows:
  default:
    plan:
      steps:
        - init
        - plan`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  pull_request_workflow: default
`,
			expErr: "repo config not allowed to set 'pull_request_workflow' key: server-side config needs 'allowed_overrides: [pull_request_workflow]'",
		},

		// If the repos are allowed to set everything then their config should
		// come through.
		"full repo permissions": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
  apply_requirements: [approved]
  allowed_overrides: [apply_requirements, pull_request_workflow]
  allow_custom_workflows: true
pull_request_workflows:
  default:
    plan:
      steps: []
  custom:
    plan:
      steps:
       - plan
`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
  apply_requirements: []
  pull_request_workflow: custom
`,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"plan"},
		},

		// Repos can choose server-side workflows.
		"repos choose server-side workflow": {
			globalCfg: `
repos:
- id: /.*/
  workflow: default
  allowed_overrides: [pull_request_workflow]
pull_request_workflows:
  default:
    plan:
      steps: []
  custom:
    plan:
      steps: [plan]
`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
  pull_request_workflow: custom
`,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"plan"},
		},

		// Test that if we leave keys undefined, that they don't override.
		"cascading matches": {
			globalCfg: `
repos:
- id: /.*/
  apply_requirements: [approved]
- id: github.com/owner/repo
  pull_request_workflow: custom
pull_request_workflows:
  custom:
    plan:
      steps: [plan]
`,
			repoCfg: `
version: 3
projects:
- dir: project1
  workspace: myworkspace
`,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -d project1 -w myworkspace",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logger,
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "",
				ApplyRequirements: []string{"approved"},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -d project1 -w myworkspace -- flag",
				RepoRelDir:        "project1",
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"plan"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, cleanup := DirStructure(t, map[string]interface{}{
				"project1": map[string]interface{}{
					"main.tf": nil,
				},
				"modules": map[string]interface{}{
					"module": map[string]interface{}{
						"main.tf": nil,
					},
				},
			})
			defer cleanup()

			workingDir := NewMockWorkingDir()
			When(workingDir.Clone(matchers.AnyLoggingLogger(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), AnyString())).ThenReturn(tmp, false, nil)
			vcsClient := vcsmocks.NewMockClient()
			When(vcsClient.GetModifiedFiles(matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest())).ThenReturn([]string{"modules/module/main.tf"}, nil)

			// Write and parse the global config file.
			globalCfgPath := filepath.Join(tmp, "global.yaml")
			Ok(t, os.WriteFile(globalCfgPath, []byte(c.globalCfg), 0600))
			parser := &config.ParserValidator{}
			globalCfg, err := parser.ParseGlobalCfg(globalCfgPath, valid.NewGlobalCfg("somedir"))
			Ok(t, err)

			if c.repoCfg != "" {
				Ok(t, os.WriteFile(filepath.Join(tmp, "atlantis.yaml"), []byte(c.repoCfg), 0600))
			}

			builder := &DefaultProjectCommandBuilder{
				ParserValidator:   &config.ParserValidator{},
				ProjectFinder:     &DefaultProjectFinder{},
				VCSClient:         vcsClient,
				WorkingDir:        workingDir,
				WorkingDirLocker:  NewDefaultWorkingDirLocker(),
				GlobalCfg:         globalCfg,
				PendingPlanFinder: &DefaultPendingPlanFinder{},
				ProjectCommandContextBuilder: &projectCommandContextBuilder{
					CommentBuilder: &CommentParser{},
				},
				AutoplanFileList: "**/*.tf,**/*.tfvars,**/*.tfvars.json,**/terragrunt.hcl",
				EnableRegExpCmd:  false,
			}

			// We run a test for each type of command.
			for _, cmd := range []command.Name{command.Plan, command.Apply} {
				t.Run(cmd.String(), func(t *testing.T) {
					ctxs, err := builder.buildProjectCommandCtx(&command.Context{
						RequestCtx: context.TODO(),
						Log:        logger,
						Pull: models.PullRequest{
							BaseRepo: baseRepo,
						},
						PullRequestStatus: models.PullReqStatus{
							Mergeable: true,
						},
					}, cmd, "", []string{"flag"}, tmp, "project1", "myworkspace", false, "")

					if c.expErr != "" {
						ErrEquals(t, c.expErr, err)
						return
					}
					ctx := ctxs[0]

					Ok(t, err)

					// Construct expected steps.
					var stepNames []string
					switch cmd {
					case command.Plan:
						stepNames = c.expPlanSteps
					}
					var expSteps []valid.Step
					for _, stepName := range stepNames {
						expSteps = append(expSteps, valid.Step{
							StepName: stepName,
						})
					}

					c.expCtx.CommandName = cmd
					// Init fields we couldn't in our cases map.
					c.expCtx.Steps = expSteps
					ctx.PolicySets = emptyPolicySets

					// Job ID cannot be compared since its generated at random
					ctx.JobID = ""

					Equals(t, c.expCtx, ctx)
					// Equals() doesn't compare TF version properly so have to
					// use .String().
					if c.expCtx.TerraformVersion != nil {
						Equals(t, c.expCtx.TerraformVersion.String(), ctx.TerraformVersion.String())
					}
				})
			}
		})
	}
}

func TestBuildProjectCmdCtx_WithRegExpCmdEnabled(t *testing.T) {
	emptyPolicySets := valid.PolicySets{
		Version:    nil,
		PolicySets: []valid.PolicySet{},
	}
	baseRepo := models.Repo{
		FullName: "owner/repo",
		VCSHost: models.VCSHost{
			Hostname: "github.com",
		},
	}
	pull := models.PullRequest{
		BaseRepo: baseRepo,
	}
	cases := map[string]struct {
		globalCfg     string
		repoCfg       string
		expErr        string
		expCtx        command.ProjectContext
		expPlanSteps  []string
		expApplySteps []string
	}{

		// Test that if we've set global defaults, that they are used but the
		// allowed project config values also come through.
		"global defaults with repo cfg": {
			globalCfg: `
repos:
- id: /.*/
  pull_request_workflow: default
pull_request_workflows:
  default:
    plan:
      steps:
      - init
      - plan`,
			repoCfg: `
version: 3
projects:
- name: myproject_1
  dir: project1
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
- name: myproject_2
  dir: project2
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
- name: myproject_3
  dir: project3
  workspace: myworkspace
  autoplan:
    enabled: true
    when_modified: [../modules/**/*.tf]
  terraform_version: v10.0
  `,
			expCtx: command.ProjectContext{
				ApplyCmd:           "atlantis apply -p myproject_1",
				BaseRepo:           baseRepo,
				EscapedCommentArgs: []string{`\f\l\a\g`},
				AutoplanEnabled:    true,
				HeadRepo:           models.Repo{},
				Log:                logging.NewNoopCtxLogger(t),
				PullReqStatus: models.PullReqStatus{
					Mergeable: true,
				},
				Pull:              pull,
				ProjectName:       "myproject_1",
				ApplyRequirements: []string{},
				RepoConfigVersion: 3,
				RePlanCmd:         "atlantis plan -p myproject_1 -- flag",
				RepoRelDir:        "project1",
				TerraformVersion:  mustVersion("10.0"),
				User:              models.User{},
				Workspace:         "myworkspace",
				PolicySets:        emptyPolicySets,
				RequestCtx:        context.TODO(),
			},
			expPlanSteps: []string{"init", "plan"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			tmp, cleanup := DirStructure(t, map[string]interface{}{
				"project1": map[string]interface{}{
					"main.tf": nil,
				},
				"modules": map[string]interface{}{
					"module": map[string]interface{}{
						"main.tf": nil,
					},
				},
			})
			defer cleanup()

			workingDir := NewMockWorkingDir()
			When(workingDir.Clone(matchers.AnyLoggingLogger(), matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest(), AnyString())).ThenReturn(tmp, false, nil)
			vcsClient := vcsmocks.NewMockClient()
			When(vcsClient.GetModifiedFiles(matchers.AnyModelsRepo(), matchers.AnyModelsPullRequest())).ThenReturn([]string{"modules/module/main.tf"}, nil)

			// Write and parse the global config file.
			globalCfgPath := filepath.Join(tmp, "global.yaml")
			Ok(t, os.WriteFile(globalCfgPath, []byte(c.globalCfg), 0600))
			parser := &config.ParserValidator{}
			globalCfg, err := parser.ParseGlobalCfg(globalCfgPath, valid.NewGlobalCfg("somedir"))
			Ok(t, err)

			if c.repoCfg != "" {
				Ok(t, os.WriteFile(filepath.Join(tmp, "atlantis.yaml"), []byte(c.repoCfg), 0600))
			}

			builder := &DefaultProjectCommandBuilder{
				ParserValidator:   &config.ParserValidator{},
				ProjectFinder:     &DefaultProjectFinder{},
				VCSClient:         vcsClient,
				WorkingDir:        workingDir,
				WorkingDirLocker:  NewDefaultWorkingDirLocker(),
				GlobalCfg:         globalCfg,
				PendingPlanFinder: &DefaultPendingPlanFinder{},
				ProjectCommandContextBuilder: &projectCommandContextBuilder{
					CommentBuilder: &CommentParser{},
				},
				AutoplanFileList: "**/*.tf,**/*.tfvars,**/*.tfvars.json,**/terragrunt.hcl",
				EnableRegExpCmd:  true,
			}

			// We run a test for each type of command, again specific projects
			for _, cmd := range []command.Name{command.Plan, command.Apply} {
				t.Run(cmd.String(), func(t *testing.T) {
					ctxs, err := builder.buildProjectCommandCtx(&command.Context{
						Pull: models.PullRequest{
							BaseRepo: baseRepo,
						},
						Log: logging.NewNoopCtxLogger(t),
						PullRequestStatus: models.PullReqStatus{
							Mergeable: true,
						},
						RequestCtx: context.TODO(),
					}, cmd, "myproject_[1-2]", []string{"flag"}, tmp, "project1", "myworkspace", false, "")

					if c.expErr != "" {
						ErrEquals(t, c.expErr, err)
						return
					}
					ctx := ctxs[0]

					Ok(t, err)

					Equals(t, 2, len(ctxs))
					// Construct expected steps.
					var stepNames []string
					switch cmd {
					case command.Plan:
						stepNames = c.expPlanSteps
					case command.Apply:
						stepNames = c.expApplySteps
					}
					var expSteps []valid.Step
					for _, stepName := range stepNames {
						expSteps = append(expSteps, valid.Step{
							StepName: stepName,
						})
					}

					c.expCtx.CommandName = cmd
					// Init fields we couldn't in our cases map.
					c.expCtx.Steps = expSteps
					ctx.PolicySets = emptyPolicySets

					// Job ID cannot be compared since its generated at random
					ctx.JobID = ""

					Equals(t, c.expCtx, ctx)
					// Equals() doesn't compare TF version properly so have to
					// use .String().
					if c.expCtx.TerraformVersion != nil {
						Equals(t, c.expCtx.TerraformVersion.String(), ctx.TerraformVersion.String())
					}
				})
			}
		})
	}
}

//nolint:unparam
func mustVersion(v string) *version.Version {
	vers, err := version.NewVersion(v)
	if err != nil {
		panic(err)
	}
	return vers
}
