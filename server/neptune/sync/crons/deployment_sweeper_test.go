package crons_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/logging"
	"github.com/runatlantis/atlantis/server/neptune/sync/crons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
)

const day = 24 * time.Hour

// sweeperDataDir builds a data dir like a worker's: deployments with one directory per run, plus siblings
// the sweeper must never touch.
type sweeperDataDir struct {
	root string
	t    *testing.T
}

func newSweeperDataDir(t *testing.T) sweeperDataDir {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "deployments"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plugin-cache", "provider"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plugin-cache", "provider", "binary"), []byte("x"), 0o600))
	return sweeperDataDir{root: root, t: t}
}

func (d sweeperDataDir) deployment(name string, age time.Duration) string {
	d.t.Helper()
	path := filepath.Join(d.root, "deployments", name)
	require.NoError(d.t, os.MkdirAll(filepath.Join(path, "repo"), 0o755))
	require.NoError(d.t, os.WriteFile(filepath.Join(path, "repo", "main.tf"), []byte("x"), 0o600))
	old := time.Now().Add(-age)
	require.NoError(d.t, os.Chtimes(path, old, old))
	return path
}

func (d sweeperDataDir) exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func newSweeper(t *testing.T, dataDir string, maxAge time.Duration, deleteStale bool) *crons.DeploymentSweeper {
	t.Helper()
	sweeper, err := crons.NewDeploymentSweeper(dataDir, maxAge, deleteStale, logging.NewNoopCtxLogger(t), tally.NoopScope)
	require.NoError(t, err)
	return sweeper
}

func TestDeploymentSweeper_DeletesOnlyStaleDirectories(t *testing.T) {
	d := newSweeperDataDir(t)
	stale := d.deployment("stale", 20*day)
	older := d.deployment("older", 200*day)
	recent := d.deployment("recent", 2*day)
	inFlight := d.deployment("in-flight", time.Minute)

	require.NoError(t, newSweeper(t, d.root, 15*day, true).Run(context.Background()))

	assert.False(t, d.exists(stale))
	assert.False(t, d.exists(older))
	assert.True(t, d.exists(recent))
	assert.True(t, d.exists(inFlight))
}

func TestDeploymentSweeper_UsesDefaultMaxAge(t *testing.T) {
	d := newSweeperDataDir(t)
	stale := d.deployment("stale", 16*day)
	recent := d.deployment("recent", 14*day)

	require.NoError(t, newSweeper(t, d.root, 0, true).Run(context.Background()))

	assert.False(t, d.exists(stale))
	assert.True(t, d.exists(recent))
}

func TestDeploymentSweeper_DryRunDeletesNothing(t *testing.T) {
	d := newSweeperDataDir(t)
	stale := d.deployment("stale", 20*day)
	recent := d.deployment("recent", 2*day)

	require.NoError(t, newSweeper(t, d.root, 15*day, false).Run(context.Background()))

	assert.True(t, d.exists(stale))
	assert.True(t, d.exists(recent))
}

func TestDeploymentSweeper_LeavesEverythingElseAlone(t *testing.T) {
	d := newSweeperDataDir(t)
	d.deployment("stale", 20*day)

	// a stray file and a symlink in the deployments dir, both old
	stray := filepath.Join(d.root, "deployments", "stray-file")
	require.NoError(t, os.WriteFile(stray, []byte("x"), 0o600))
	// a symlink to a directory outside deployments must be left alone, and so must its target
	target := filepath.Join(d.root, "plugin-cache")
	link := filepath.Join(d.root, "deployments", "link")
	require.NoError(t, os.Symlink(target, link))
	old := time.Now().Add(-100 * day)
	require.NoError(t, os.Chtimes(stray, old, old))

	require.NoError(t, newSweeper(t, d.root, 15*day, true).Run(context.Background()))

	assert.True(t, d.exists(stray))
	assert.True(t, d.exists(link))
	assert.True(t, d.exists(filepath.Join(target, "provider", "binary")))
	assert.True(t, d.exists(filepath.Join(d.root, "deployments")))
}

func TestDeploymentSweeper_OldDeploymentsDirItselfIsNotDeleted(t *testing.T) {
	d := newSweeperDataDir(t)
	deployments := filepath.Join(d.root, "deployments")
	old := time.Now().Add(-365 * day)
	require.NoError(t, os.Chtimes(deployments, old, old))

	require.NoError(t, newSweeper(t, d.root, 15*day, true).Run(context.Background()))

	assert.True(t, d.exists(deployments))
}

func TestDeploymentSweeper_MissingDeploymentsDir(t *testing.T) {
	root := t.TempDir()

	assert.NoError(t, newSweeper(t, root, 15*day, true).Run(context.Background()))
}

func TestDeploymentSweeper_StopsWhenContextCancelled(t *testing.T) {
	d := newSweeperDataDir(t)
	stale := d.deployment("stale", 20*day)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, newSweeper(t, d.root, 15*day, true).Run(ctx))

	assert.True(t, d.exists(stale))
}

func TestDeploymentSweeper_ContinuesAfterDeleteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions are not enforced for root")
	}
	d := newSweeperDataDir(t)
	// a directory that can't be emptied because its child directory is read-only
	stuck := d.deployment("a-stuck", 20*day)
	locked := filepath.Join(stuck, "repo")
	require.NoError(t, os.Chmod(locked, 0o500))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	other := d.deployment("z-other", 20*day)

	require.NoError(t, newSweeper(t, d.root, 15*day, true).Run(context.Background()))

	assert.True(t, d.exists(stuck))
	assert.False(t, d.exists(other))
}

func TestNewDeploymentSweeper_RejectsUnsafeConfig(t *testing.T) {
	cases := []struct {
		description string
		dataDir     string
		maxAge      time.Duration
	}{
		{description: "empty data dir", dataDir: "", maxAge: 15 * day},
		{description: "root data dir", dataDir: "/", maxAge: 15 * day},
		{description: "max age below the minimum", dataDir: t.TempDir(), maxAge: time.Hour},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			_, err := crons.NewDeploymentSweeper(c.dataDir, c.maxAge, true, logging.NewNoopCtxLogger(t), tally.NoopScope)
			assert.Error(t, err)
		})
	}
}

func TestNewDeploymentSweeper_AcceptsMinimumMaxAge(t *testing.T) {
	_, err := crons.NewDeploymentSweeper(t.TempDir(), crons.MinDeploymentMaxAge, true, logging.NewNoopCtxLogger(t), tally.NoopScope)
	assert.NoError(t, err)
}
