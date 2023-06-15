package runtime

import (
	"context"
	"testing"

	"github.com/hashicorp/go-version"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/legacy/core/terraform/mocks"
	"github.com/runatlantis/atlantis/server/legacy/events/command"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	. "github.com/runatlantis/atlantis/testing"
)

func TestRunVersionStep(t *testing.T) {
	RegisterMockTestingT(t)
	logger := logging.NewNoopCtxLogger(t)
	workspace := "default"

	ctx := context.Background()
	prjCtx := command.ProjectContext{
		Log:                logger,
		EscapedCommentArgs: []string{"comment", "args"},
		Workspace:          workspace,
		RepoRelDir:         ".",
		User:               models.User{Username: "username"},
		Pull: models.PullRequest{
			Num: 2,
		},
		BaseRepo: models.Repo{
			FullName: "owner/repo",
			Owner:    "owner",
			Name:     "repo",
		},
	}

	terraform := mocks.NewMockClient()
	tfVersion, _ := version.NewVersion("0.15.0")
	tmpDir, cleanup := TempDir(t)
	defer cleanup()

	s := &VersionStepRunner{
		TerraformExecutor: terraform,
		DefaultTFVersion:  tfVersion,
	}

	t.Run("ensure runs", func(t *testing.T) {
		_, err := s.Run(ctx, prjCtx, []string{}, tmpDir, map[string]string(nil))
		terraform.VerifyWasCalledOnce().RunCommandWithVersion(ctx, prjCtx, tmpDir, []string{"version"}, map[string]string(nil), tfVersion, "default")
		Ok(t, err)
	})
}
