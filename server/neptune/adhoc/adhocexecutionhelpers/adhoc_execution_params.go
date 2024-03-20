package adhoc

import (
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/github"
	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform"
)

type AdhocTerraformWorkflowExecutionParams struct {
	AtlantisRoot  string
	AtlantisRepo  string
	Revision      string
	TerraformRoot terraform.Root
	GithubRepo    github.Repo
	// Note that deploymentID is used in NewWorkflowStore(), but we don't care about that in adhoc mode so can leave it blank
}

func getAdhocExecutionParams(config *adhocconfig.Config) AdhocTerraformWorkflowExecutionParams {
	return AdhocTerraformWorkflowExecutionParams{
		AtlantisRoot:  "",
		AtlantisRepo:  "",
		Revision:      "",
		TerraformRoot: terraform.Root{},
		GithubRepo:    github.Repo{},
	}
}
