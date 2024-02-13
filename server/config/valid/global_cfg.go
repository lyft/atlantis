package valid

import (
	"regexp"

	"github.com/graymeta/stow"
	"github.com/graymeta/stow/local"
	stow_s3 "github.com/graymeta/stow/s3"
	version "github.com/hashicorp/go-version"
	"github.com/runatlantis/atlantis/server/logging"
)

const (
	LocalStore               = "artifact-store"
	DefaultJobsPrefix        = "jobs"
	DefaultDeploymentsPrefix = "deployments"
)

const MergeableApplyReq = "mergeable"
const ApprovedApplyReq = "approved"
const UnDivergedApplyReq = "undiverged"
const SQUnlockedApplyReq = "unlocked"
const PoliciesPassedApplyReq = "policies_passed"
const ApplyRequirementsKey = "apply_requirements"
const WorkflowKey = "workflow"
const PullRequestWorkflowKey = "pull_request_workflow"
const DeploymentWorkflowKey = "deployment_workflow"
const AllowedOverridesKey = "allowed_overrides"
const AllowCustomWorkflowsKey = "allow_custom_workflows"

const DefaultWorkflowName = "default"

// NonOverrideableApplyReqs will get applied across all "repos" in the server side config.
// If repo config is allowed overrides, they can override this.
// TODO: Make this more customizable, not everyone wants this rigid workflow
// maybe something along the lines of defining overridable/non-overrideable apply
// requirements in the config and removing the flag to enable policy checking.
var NonOverrideableApplyReqs = []string{PoliciesPassedApplyReq}

type WorkflowModeType int

const (
	PlatformWorkflowMode WorkflowModeType = iota
	DefaultWorkflowMode
)

type BackendType string

const (
	S3Backend    BackendType = "s3"
	LocalBackend BackendType = "local"
)

// GlobalCfg is the final parsed version of server-side repo config.
type GlobalCfg struct {
	Repos                []Repo
	PullRequestWorkflows map[string]Workflow
	DeploymentWorkflows  map[string]Workflow
	PolicySets           PolicySets
	Metrics              Metrics
	PersistenceConfig    PersistenceConfig
	TerraformLogFilter   TerraformLogFilters
	Temporal             Temporal
	Github               Github
	RevisionSetter       RevisionSetter
	Admin                Admin
	TerraformAdminMode   TerraformAdminMode
}

type TerraformAdminMode struct {
	Repo string
	Root string
}

type GithubTeam struct {
	Name string
	Org  string
}

type Admin struct {
	GithubTeam GithubTeam
}

type PersistenceConfig struct {
	Deployments StoreConfig
	Jobs        StoreConfig
}

type StoreConfig struct {
	ContainerName string
	Prefix        string
	BackendType   BackendType
	Config        stow.Config
}

// Interface to configure the storage backends
// Additional storage backends will implement this interface
type BackendConfigurer interface {
	GetConfigMap() stow.Config
	GetConfiguredBackend() string
	GetContainerName() string
}

type Jobs struct {
	StorageBackend *StorageBackend
}

type StorageBackend struct {
	BackendConfig BackendConfigurer
}

// S3 implementation for s3 backend storage
type S3 struct {
	BucketName string
}

func (s *S3) GetConfigMap() stow.Config {
	// Only supports Iam auth type for now
	// TODO: Add accesskeys auth type
	return stow.ConfigMap{
		stow_s3.ConfigAuthType: "iam",
	}
}

func (s *S3) GetConfiguredBackend() string {
	return "s3"
}

func (s *S3) GetContainerName() string {
	return s.BucketName
}

type Metrics struct {
	Statsd *Statsd
	Log    *Log
}

type Log struct{}

type Statsd struct {
	Port         string
	Host         string
	TagSeparator string
}

type Temporal struct {
	Port               string
	Host               string
	UseSystemCACert    bool
	Namespace          string
	TerraformTaskQueue string
}

type Github struct {
	GatewayAppInstallationID  int64
	TemporalAppInstallationID int64
}

type TerraformLogFilters struct {
	Regexes []*regexp.Regexp
}

type BasicAuth struct {
	Username string
	Password string
}

type TaskQueue struct {
	ActivitiesPerSecond float64
}

type RevisionSetter struct {
	BasicAuth BasicAuth
	URL       string

	DefaultTaskQueue TaskQueue
	SlowTaskQueue    TaskQueue
}

// TODO: rename project to roots
type MergedProjectCfg struct {
	ApplyRequirements   []string
	PullRequestWorkflow Workflow
	DeploymentWorkflow  Workflow
	AllowedWorkflows    []string
	RepoRelDir          string
	Workspace           string
	Name                string
	AutoplanEnabled     bool
	WhenModified        []string
	TerraformVersion    *version.Version
	RepoCfgVersion      int
	PolicySets          PolicySets
	Tags                map[string]string
}

// PreWorkflowHook is a map of custom run commands to run before workflows.
type PreWorkflowHook struct {
	StepName   string
	RunCommand string
}

// DefaultApplyStage is the Atlantis default apply stage.
var DefaultApplyStage = Stage{
	Steps: []Step{
		{
			StepName: "apply",
		},
	},
}

// DefaultPolicyCheckStage is the Atlantis default policy check stage.
var DefaultPolicyCheckStage = Stage{
	Steps: []Step{
		{
			StepName: "show",
		},
		{
			StepName: "policy_check",
		},
	},
}

// DefaultPlanStage is the Atlantis default plan stage.
var DefaultPlanStage = Stage{
	Steps: []Step{
		{
			StepName: "init",
		},
		{
			StepName: "plan",
		},
	},
}

// DefaultLocklessPlanStage is the Atlantis default plan stage for PR workflows in
// platform mode.
var DefaultLocklessPlanStage = Stage{
	Steps: []Step{
		{
			StepName: "init",
		},
		{
			StepName:  "plan",
			ExtraArgs: []string{"-lock=false"},
		},
	},
}

func NewGlobalCfg(dataDir string) GlobalCfg {
	// defaultPullRequstWorkflow is only used in platform mode. By default it does not
	// support apply stage, and plan stage run with -lock=false flag
	pullRequestWorkflow := Workflow{
		Name:        DefaultWorkflowName,
		Plan:        DefaultLocklessPlanStage,
		PolicyCheck: DefaultPolicyCheckStage,
	}

	deploymentWorkflow := Workflow{
		Name:  DefaultWorkflowName,
		Apply: DefaultApplyStage,
		Plan:  DefaultPlanStage,
	}

	repo := Repo{
		IDRegex:             regexp.MustCompile(".*"),
		BranchRegex:         regexp.MustCompile(".*"),
		DeploymentWorkflow:  &deploymentWorkflow,
		PullRequestWorkflow: &pullRequestWorkflow,
		AllowedWorkflows:    []string{},
		ApplyRequirements:   []string{},
		AllowedOverrides:    []string{},
		CheckoutStrategy:    "branch",
		ApplySettings: ApplySettings{
			BranchRestriction: DefaultBranchRestriction,
		},
	}

	globalCfg := GlobalCfg{
		DeploymentWorkflows: map[string]Workflow{
			DefaultWorkflowName: deploymentWorkflow,
		},
		PullRequestWorkflows: map[string]Workflow{
			DefaultWorkflowName: pullRequestWorkflow,
		},
	}

	globalCfg.PersistenceConfig = PersistenceConfig{
		Deployments: StoreConfig{
			BackendType: LocalBackend,
			Prefix:      DefaultDeploymentsPrefix,
			Config: stow.ConfigMap{
				local.ConfigKeyPath: dataDir,
			},
			ContainerName: LocalStore,
		},
		Jobs: StoreConfig{
			BackendType: LocalBackend,
			Prefix:      DefaultJobsPrefix,
			Config: stow.ConfigMap{
				local.ConfigKeyPath: dataDir,
			},
			ContainerName: LocalStore,
		},
	}

	globalCfg.Repos = []Repo{repo}

	return globalCfg
}

// MergeProjectCfg merges proj and rCfg with the global config to return a
// final config. It assumes that all configs have been validated.
func (g GlobalCfg) MergeProjectCfg(repoID string, proj Project, rCfg RepoCfg) MergedProjectCfg {
	var applyReqs []string
	var pullRequestWorkflow Workflow
	var deploymentWorkflow Workflow

	repo := g.foldMatchingRepos(repoID)

	applyReqs = repo.ApplyRequirements

	pullRequestWorkflow = *repo.PullRequestWorkflow
	deploymentWorkflow = *repo.DeploymentWorkflow

	// If repos are allowed to override certain keys then override them.
	for _, key := range repo.AllowedOverrides {
		switch key {
		case ApplyRequirementsKey:
			if proj.ApplyRequirements != nil {
				applyReqs = proj.ApplyRequirements
			}
		case PullRequestWorkflowKey:
			if proj.PullRequestWorkflowName != nil {
				name := *proj.PullRequestWorkflowName
				// We iterate over the global workflows first and the repo
				// workflows second so that repo workflows override. This is
				// safe because at this point we know if a repo is allowed to
				// define its own workflow. We also know that a workflow will
				// exist with this name due to earlier validation.
				if w, ok := g.PullRequestWorkflows[name]; ok {
					pullRequestWorkflow = w
				}
			}
		case DeploymentWorkflowKey:
			if proj.DeploymentWorkflowName != nil {
				name := *proj.DeploymentWorkflowName
				if w, ok := g.DeploymentWorkflows[name]; ok {
					deploymentWorkflow = w
				}
			}
		}
	}

	return MergedProjectCfg{
		ApplyRequirements:   applyReqs,
		PullRequestWorkflow: pullRequestWorkflow,
		DeploymentWorkflow:  deploymentWorkflow,
		RepoRelDir:          proj.Dir,
		Workspace:           proj.Workspace,
		Name:                proj.GetName(),
		AutoplanEnabled:     proj.Autoplan.Enabled,
		WhenModified:        proj.Autoplan.WhenModified,
		TerraformVersion:    proj.TerraformVersion,
		RepoCfgVersion:      rCfg.Version,
		PolicySets:          g.PolicySets,
		Tags:                proj.Tags,
	}
}

// DefaultProjCfg returns the default project config for all projects under the
// repo with id repoID. It is used when there is no repo config.
func (g GlobalCfg) DefaultProjCfg(log logging.Logger, repoID string, repoRelDir string, workspace string) MergedProjectCfg {
	repo := g.foldMatchingRepos(repoID)

	mrgPrj := MergedProjectCfg{
		ApplyRequirements:   repo.ApplyRequirements,
		PullRequestWorkflow: *repo.PullRequestWorkflow,
		DeploymentWorkflow:  *repo.DeploymentWorkflow,
		RepoRelDir:          repoRelDir,
		Workspace:           workspace,
		Name:                "",
		AutoplanEnabled:     DefaultAutoPlanEnabled,
		TerraformVersion:    nil,
		PolicySets:          g.PolicySets,
	}

	return mrgPrj
}

// foldMatchingRepos will return a pseudo repo instance that will iterate over
// the matching repositories and assign relevant fields if they're defined.
// This means returned object will contain the last matching repo's value as a it's fields
func (g GlobalCfg) foldMatchingRepos(repoID string) Repo {
	foldedRepo := Repo{
		AllowedWorkflows:  make([]string, 0),
		AllowedOverrides:  make([]string, 0),
		ApplyRequirements: make([]string, 0),
	}

	for _, repo := range g.Repos {
		if repo.IDMatches(repoID) {
			if repo.ApplyRequirements != nil {
				foldedRepo.ApplyRequirements = repo.ApplyRequirements
			}
			if repo.PullRequestWorkflow != nil {
				foldedRepo.PullRequestWorkflow = repo.PullRequestWorkflow
			}
			if repo.DeploymentWorkflow != nil {
				foldedRepo.DeploymentWorkflow = repo.DeploymentWorkflow
			}
			if repo.AllowedWorkflows != nil {
				foldedRepo.AllowedWorkflows = repo.AllowedWorkflows
			}
			if repo.AllowedOverrides != nil {
				foldedRepo.AllowedOverrides = repo.AllowedOverrides
			}
		}
	}

	return foldedRepo
}

// ValidateRepoCfg validates that rCfg for repo with id repoID is valid based
// on our global config.
func (g GlobalCfg) ValidateRepoCfg(rCfg RepoCfg, repoID string) error {
	repo := g.foldMatchingRepos(repoID)

	// Check allowed overrides.
	allowedOverrides := repo.AllowedOverrides

	if err := rCfg.ValidateAllowedOverrides(allowedOverrides); err != nil {
		return err
	}

	// Check if the repo has set a workflow name that doesn't exist and if workflow is allowed
	if err := rCfg.ValidatePRWorkflows(g.PullRequestWorkflows, repo.AllowedWorkflows); err != nil {
		return err
	}
	if err := rCfg.ValidateDeploymentWorkflows(g.DeploymentWorkflows, repo.AllowedWorkflows); err != nil {
		return err
	}

	return nil
}

// MatchingRepo returns an instance of Repo which matches a given repoID.
// If multiple repos match, return the last one for consistency with getMatchingCfg.
func (g GlobalCfg) MatchingRepo(repoID string) *Repo {
	for i := len(g.Repos) - 1; i >= 0; i-- {
		repo := g.Repos[i]
		if repo.IDMatches(repoID) {
			return &repo
		}
	}
	return nil
}
