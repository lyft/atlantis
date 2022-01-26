package decorators_test

import (
	"github.com/runatlantis/atlantis/server/lyft/decorators"
	"testing"

	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/yaml/valid"
	"github.com/runatlantis/atlantis/server/logging"
	. "github.com/runatlantis/atlantis/testing"
)

func TestDestroyPlanProjectFinder_DetermineProjectsViaConfig(t *testing.T) {
	// Create dir structure:
	// main.tf
	// project1/
	//   main.tf
	//   terraform.tfvars.json
	// project2/
	//   main.tf
	//   terraform.tfvars
	tmpDir, cleanup := DirStructure(t, map[string]interface{}{
		"main.tf": nil,
		"project1": map[string]interface{}{
			"main.tf":               nil,
			"terraform.tfvars.json": nil,
		},
		"project2": map[string]interface{}{
			"main.tf":          nil,
			"terraform.tfvars": nil,
		},
	})
	defer cleanup()

	cases := []struct {
		description  string
		config       valid.RepoCfg
		modified     []string
		expProjPaths []string
	}{
		{
			description: "destroy plans enabled",
			config: valid.RepoCfg{
				Projects: []valid.Project{
					{
						Dir: ".",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
						Tags: map[string]string{
							decorators.PlanMode: decorators.Destroy,
						},
					},
					{
						Dir: "project1",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
						Tags: map[string]string{
							decorators.PlanMode: decorators.Destroy,
						},
					},
					{
						Dir: "project2",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
						Tags: map[string]string{
							decorators.PlanMode: decorators.Destroy,
						},
					},
				},
			},
			modified:     []string{},
			expProjPaths: []string{".", "project1", "project2"},
		},
		{
			description: "no destroy plans",
			config: valid.RepoCfg{
				Projects: []valid.Project{
					{
						Dir: ".",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
					},
					{
						Dir: "project1",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
					},
					{
						Dir: "project2",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
					},
				},
			},
			modified:     []string{},
			expProjPaths: []string{},
		},
		{
			description: "partial destroy plans enabled",
			config: valid.RepoCfg{
				Projects: []valid.Project{
					{
						Dir: ".",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
					},
					{
						Dir: "project1",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
						Tags: map[string]string{
							decorators.PlanMode: decorators.Destroy,
						},
					},
					{
						Dir: "project2",
						Autoplan: valid.Autoplan{
							Enabled:      true,
							WhenModified: []string{"main.tf"},
						},
					},
				},
			},
			modified:     []string{},
			expProjPaths: []string{"project1"},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			destroyPlanProjectFinder := decorators.DestroyPlanProjectFinderWrapper{
				ProjectFinder: &events.DefaultProjectFinder{},
			}
			projects, err := destroyPlanProjectFinder.DetermineProjectsViaConfig(logging.NewNoopLogger(t), c.modified, c.config, tmpDir)
			Ok(t, err)
			Equals(t, len(c.expProjPaths), len(projects))
			for i, proj := range projects {
				Equals(t, c.expProjPaths[i], proj.Dir)
			}
		})
	}
}
