package github

import (
	"context"
	"errors"
	"github.com/runatlantis/atlantis/server/events/models"
)

type MockSuccessFileFetcher struct{}

func (m *MockSuccessFileFetcher) GetModifiedFilesFromCommit(_ context.Context, _ models.Repo, _ string, _ int64) ([]string, error) {
	return nil, nil
}

type MockFailureFileFetcher struct{}

func (m *MockFailureFileFetcher) GetModifiedFilesFromCommit(_ context.Context, _ models.Repo, _ string, _ int64) ([]string, error) {
	return nil, errors.New("some error")
}
