package valid

import (
	"fmt"

	version "github.com/hashicorp/go-version"
)

const (
	DefaultWorkflowType     = "default"
	PullRequestWorkflowType = "pull_request"
	DeploymentWorkflowType  = "deployment"
)

type Project struct {
	Dir                       string
	Workspace                 string
	Name                      *string
	WorkflowName              *string
	PullRequestWorkflowName   *string
	DeploymentWorkflowName    *string
	TerraformVersion          *version.Version
	Autoplan                  Autoplan
	ApplyRequirements         []string
	DeleteSourceBranchOnMerge *bool
	Tags                      map[string]string
}

// GetName returns the name of the project or an empty string if there is no
// project name.
func (p Project) GetName() string {
	if p.Name != nil {
		return *p.Name
	}
	// TODO
	// Upstream atlantis only requires project name to be set if there's more than one project
	// with same dir and workspace. If a project name has not been set, we'll use the dir and
	// workspace to build project key.
	// Source: https://www.runatlantis.io/docs/repo-level-atlantis-yaml.html#reference
	return ""
}

func (p Project) CheckAllowedOverrides(allowedOverrides []string) error {
	if p.WorkflowName != nil && !sliceContains(allowedOverrides, WorkflowKey) {
		return fmt.Errorf("repo config not allowed to set '%s' key: server-side config needs '%s: [%s]'", WorkflowKey, AllowedOverridesKey, WorkflowKey)
	}
	if p.PullRequestWorkflowName != nil && !sliceContains(allowedOverrides, PullRequestWorkflowKey) {
		return fmt.Errorf("repo config not allowed to set '%s' key: server-side config needs '%s: [%s]'", PullRequestWorkflowKey, AllowedOverridesKey, PullRequestWorkflowKey)
	}
	if p.DeploymentWorkflowName != nil && !sliceContains(allowedOverrides, DeploymentWorkflowKey) {
		return fmt.Errorf("repo config not allowed to set '%s' key: server-side config needs '%s: [%s]'", DeploymentWorkflowKey, AllowedOverridesKey, DeploymentWorkflowKey)
	}
	if p.ApplyRequirements != nil && !sliceContains(allowedOverrides, ApplyRequirementsKey) {
		return fmt.Errorf("repo config not allowed to set '%s' key: server-side config needs '%s: [%s]'", ApplyRequirementsKey, AllowedOverridesKey, ApplyRequirementsKey)
	}
	if p.DeleteSourceBranchOnMerge != nil && !sliceContains(allowedOverrides, DeleteSourceBranchOnMergeKey) {
		return fmt.Errorf("repo config not allowed to set '%s' key: server-side config needs '%s: [%s]'", DeleteSourceBranchOnMergeKey, AllowedOverridesKey, DeleteSourceBranchOnMergeKey)
	}

	return nil
}

func (p Project) ValidateWorkflow(repoWorkflows map[string]Workflow, globalWorkflows map[string]Workflow) error {
	if p.WorkflowName != nil {
		name := *p.WorkflowName

		if !mapContains(repoWorkflows, name) && !mapContains(globalWorkflows, name) {
			return fmt.Errorf("workflow %q is not defined anywhere", name)
		}
	}

	return nil
}

func (p Project) ValidatePRWorkflow(globalWorkflows map[string]Workflow) error {
	if p.PullRequestWorkflowName != nil {
		name := *p.PullRequestWorkflowName

		if !mapContains(globalWorkflows, name) {
			return fmt.Errorf("pull_request_workflow %q is not defined anywhere", name)
		}
	}

	return nil
}

func (p Project) ValidateDeploymentWorkflow(globalWorkflows map[string]Workflow) error {
	if p.DeploymentWorkflowName != nil {
		name := *p.DeploymentWorkflowName

		if !mapContains(globalWorkflows, name) {
			return fmt.Errorf("deployment_workflow %q is not defined anywhere", name)
		}
	}

	return nil
}

func (p Project) ValidateWorkflowAllowed(allowedWorkflows []string) error {
	if p.WorkflowName != nil {
		name := *p.WorkflowName

		if !sliceContains(allowedWorkflows, name) {
			return fmt.Errorf("workflow %q is not allowed for this repo", name)
		}
	}

	return nil
}

func (p Project) ValidatePRWorkflowAllowed(allowedWorkflows []string) error {
	if p.PullRequestWorkflowName != nil {
		name := *p.PullRequestWorkflowName

		if !sliceContains(allowedWorkflows, name) {
			return fmt.Errorf("pull_request_workflow %q is not allowed for this repo", name)
		}
	}

	return nil
}

func (p Project) ValidateDeploymentWorkflowAllowed(allowedWorkflows []string) error {
	if p.DeploymentWorkflowName != nil {
		name := *p.DeploymentWorkflowName

		if !sliceContains(allowedWorkflows, name) {
			return fmt.Errorf("deployment_workflow %q is not allowed for this repo", name)
		}
	}

	return nil
}

// helper function to check if string is in array
func sliceContains(slc []string, str string) bool {
	for _, s := range slc {
		if s == str {
			return true
		}
	}
	return false
}

// helper function to check if map contains a key
func mapContains(m map[string]Workflow, key string) bool {
	_, ok := m[key]
	return ok
}
