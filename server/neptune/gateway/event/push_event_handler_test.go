package event_test

import (
	"context"
	"testing"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/models"
	"github.com/runatlantis/atlantis/server/neptune/gateway/deploy"
	"github.com/runatlantis/atlantis/server/neptune/gateway/event"
	"github.com/runatlantis/atlantis/server/neptune/sync"
	"github.com/runatlantis/atlantis/server/vcs"
	"github.com/stretchr/testify/assert"
)

const testRoot = "testroot"

func TestHandlePushEvent_FiltersEvents(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)
	repoFullName := "nish/repo"
	repoOwner := "nish"
	repoName := "repo"
	repoURL := "www.nish.com"
	sha := "12345"

	t.Run("filters non branch types", func(t *testing.T) {
		e := event.Push{
			Repo: models.Repo{
				FullName: repoFullName,
				Name:     repoName,
				Owner:    repoOwner,
				CloneURL: repoURL,
			},
			Sha: sha,
			Ref: vcs.Ref{
				Type: vcs.TagRef,
				Name: "blah",
			},
		}
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("filters non-default branch types", func(t *testing.T) {
		e := event.Push{
			Repo: models.Repo{
				FullName:      repoFullName,
				Name:          repoName,
				Owner:         repoOwner,
				CloneURL:      repoURL,
				DefaultBranch: "main",
			},
			Sha: sha,
			Ref: vcs.Ref{
				Type: vcs.BranchRef,
				Name: "random",
			},
		}

		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("filters deleted branches", func(t *testing.T) {
		e := event.Push{
			Repo: models.Repo{
				FullName:      repoFullName,
				Name:          repoName,
				Owner:         repoOwner,
				CloneURL:      repoURL,
				DefaultBranch: "main",
			},
			Sha:    sha,
			Action: event.DeletedAction,
			Ref: vcs.Ref{
				Type: vcs.BranchRef,
				Name: "main",
			},
		}
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(context.Background(), e)
		assert.NoError(t, err)
	})
}

func TestHandlePushEvent(t *testing.T) {
	logger := logging.NewNoopCtxLogger(t)

	repoFullName := "nish/repo"
	repoOwner := "nish"
	repoName := "repo"
	repoURL := "www.nish.com"
	sha := "12345"
	repo := models.Repo{
		FullName:      repoFullName,
		Name:          repoName,
		Owner:         repoOwner,
		CloneURL:      repoURL,
		DefaultBranch: "main",
	}

	e := event.Push{
		Repo: repo,
		Ref: vcs.Ref{
			Type: vcs.BranchRef,
			Name: "main",
		},
		Sha: sha,
	}

	t.Run("allocation result false", func(t *testing.T) {
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("allocation error", func(t *testing.T) {
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(context.Background(), e)
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{},
		}

		err := handler.Handle(ctx, e)
		assert.NoError(t, err)
	})

	t.Run("root deployer error", func(t *testing.T) {
		ctx := context.Background()
		handler := event.PushHandler{
			Scheduler:    &sync.SynchronousScheduler{Logger: logger},
			Logger:       logger,
			RootDeployer: &mockRootDeployer{error: assert.AnError},
		}

		err := handler.Handle(ctx, e)
		assert.Error(t, err)
	})
}

type mockRootDeployer struct {
	isCalled bool
	error    error
}

func (m *mockRootDeployer) Deploy(_ context.Context, _ deploy.RootDeployOptions) error {
	m.isCalled = true
	return m.error
}
