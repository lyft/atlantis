package command_test

import (
	"context"
	"fmt"
	"github.com/runatlantis/atlantis/server/events/command"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/lyft/feature"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLegacyDeprecationVCSStatusUpdater_NotAllocated(t *testing.T) {
	testID := "abc"
	delegate := &testDelegate{ID: testID}
	subject := command.LegacyDeprecationVCSStatusUpdater{
		Allocator: &testFeatureAllocator{},
		Delegate:  delegate,
	}
	id, err := subject.UpdateCombinedCount(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, 0, 0, "")
	assert.NoError(t, err)
	assert.Equal(t, id, testID)
	assert.True(t, delegate.Called)

	id, err = subject.UpdateCombined(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, "", "")
	assert.NoError(t, err)
	assert.Equal(t, id, testID)
	assert.True(t, delegate.Called)

	id, err = subject.UpdateProject(context.Background(), command.ProjectContext{}, command.Plan, models.QueuedVCSStatus, "", "")
	assert.NoError(t, err)
	assert.Equal(t, id, testID)
	assert.True(t, delegate.Called)
}

func TestLegacyDeprecationVCSStatusUpdater_Allocated(t *testing.T) {
	delegate := &testDelegate{}
	subject := command.LegacyDeprecationVCSStatusUpdater{
		Allocator: &testFeatureAllocator{Enabled: true},
		Delegate:  delegate,
	}
	id, err := subject.UpdateCombinedCount(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, 0, 0, "")
	assert.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)

	id, err = subject.UpdateCombined(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, "", "")
	assert.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)

	id, err = subject.UpdateProject(context.Background(), command.ProjectContext{}, command.Plan, models.QueuedVCSStatus, "", "")
	assert.NoError(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)
}

func TestLegacyDeprecationVCSStatusUpdater_AllocatorError(t *testing.T) {
	delegate := &testDelegate{}
	subject := command.LegacyDeprecationVCSStatusUpdater{
		Allocator: &testFeatureAllocator{Err: assert.AnError},
		Delegate:  delegate,
	}
	id, err := subject.UpdateCombinedCount(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, 0, 0, "")
	assert.Error(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)

	id, err = subject.UpdateCombined(context.Background(), models.Repo{}, models.PullRequest{}, models.QueuedVCSStatus, command.Plan, "", "")
	assert.Error(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)

	id, err = subject.UpdateProject(context.Background(), command.ProjectContext{}, command.Plan, models.QueuedVCSStatus, "", "")
	assert.Error(t, err)
	assert.Empty(t, id)
	assert.False(t, delegate.Called)
}

type testDelegate struct {
	ID     string
	Err    error
	Called bool
}

func (d *testDelegate) UpdateCombined(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, statusID string, output string) (string, error) {
	d.Called = true
	return d.ID, d.Err
}

func (d *testDelegate) UpdateCombinedCount(ctx context.Context, repo models.Repo, pull models.PullRequest, status models.VCSStatus, cmdName fmt.Stringer, numSuccess int, numTotal int, statusID string) (string, error) {
	d.Called = true
	return d.ID, d.Err
}

func (d *testDelegate) UpdateProject(ctx context.Context, projectCtx command.ProjectContext, cmdName fmt.Stringer, status models.VCSStatus, url string, statusID string) (string, error) {
	d.Called = true
	return d.ID, d.Err
}

type testFeatureAllocator struct {
	Enabled bool
	Err     error
}

func (t *testFeatureAllocator) ShouldAllocate(featureID feature.Name, featureCtx feature.FeatureContext) (bool, error) {
	return t.Enabled, t.Err
}
