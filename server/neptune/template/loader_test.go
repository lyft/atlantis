package template

import (
	"os"
	"testing"

	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/stretchr/testify/assert"
)

var testRepo = models.Repo{
	VCSHost: models.VCSHost{
		Hostname: models.Github.String(),
	},
	FullName: "test-repo",
}

func TestLoader_TemplateOverride(t *testing.T) {
	globalCfg := valid.GlobalCfg{
		Repos: []valid.Repo{
			{
				ID: testRepo.ID(),
				TemplateOverrides: map[string]string{
					string(PRComment): "testdata/custom.tmpl",
				},
			},
		},
	}

	loader := NewLoader[any](globalCfg)

	output, err := loader.Load(PRComment, testRepo, nil)
	assert.NoError(t, err)

	templateContent, err := os.ReadFile(globalCfg.MatchingRepo(testRepo.ID()).TemplateOverrides[string(PRComment)])
	assert.NoError(t, err)

	assert.Equal(t, output, string(templateContent))
}
