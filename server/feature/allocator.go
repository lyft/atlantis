package feature

import (
	"context"

	"github.com/pkg/errors"
	"github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/ffuser"
)

const Configuration StringRetriever = `log-streaming:
  true: true
  false: false
  default: false
  trackEvents: false`

type StringRetriever string

func (s StringRetriever) Retrieve(ctx context.Context) ([]byte, error) {
	return []byte(s), nil
}

//go:generate pegomock generate -m --use-experimental-model-gen --package mocks -o mocks/mock_allocator.go Allocator
type Allocator interface {
	ShouldAllocate(featureID Name, fullRepoName string) (bool, error)
}

type RepoAllocator struct{}

func NewFileRepoAllocator(filepath string) (Allocator, error) {
	err := ffclient.Init(
		ffclient.Config{
			Context:   context.Background(),
			Retriever: &ffclient.FileRetriever{Path: filepath},
		},
	)

	if err != nil {
		return nil, errors.Wrapf(err, "initializing feature allocator")
	}

	return &RepoAllocator{}, err

}

func NewRepoAllocator() (Allocator, error) {
	err := ffclient.Init(
		ffclient.Config{
			Context: context.Background(),
			// TODO Change to an external source since polling this does nothing
			Retriever: Configuration,
		},
	)

	if err != nil {
		return nil, errors.Wrapf(err, "initializing feature allocator")
	}

	return &RepoAllocator{}, err
}

func (r *RepoAllocator) ShouldAllocate(featureID Name, fullRepoName string) (bool, error) {
	repo := ffuser.NewUser(fullRepoName)
	shouldAllocate, err := ffclient.BoolVariation(string(featureID), repo, false)

	// if we error out we shouldn't enable the feature, could be risky
	// TODO: what if feature doesn't exist?
	if err != nil {
		return false, errors.Wrapf(err, "getting feature %s", featureID)
	}

	return shouldAllocate, nil
}
