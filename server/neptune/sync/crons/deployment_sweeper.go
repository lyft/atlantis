package crons

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/logging"
	"github.com/uber-go/tally/v4"
)

const (
	// deploymentsDirName must match the directory GithubFetchRoot creates deployment directories in.
	deploymentsDirName = "deployments"

	// DefaultDeploymentMaxAge is how old a deployment directory must be before it is considered orphaned.
	// Terraform runs finish in minutes (an apply can take up to an hour), so a directory this old can't be in use.
	DefaultDeploymentMaxAge = 15 * 24 * time.Hour

	// MinDeploymentMaxAge guards against a misconfiguration deleting the working directories of runs in flight.
	MinDeploymentMaxAge = 24 * time.Hour
)

// DeploymentSweeper deletes deployment directories that were left behind on this worker's volume.
//
// Each Terraform run extracts the repo into <DataDir>/deployments/<deployment id> and deletes it when the run
// finishes. A run that never reaches its cleanup (a failed or interrupted fetch, a killed pod) leaves the
// directory behind and nothing else removes it, so the volume slowly fills up. The sweeper is the backstop
// for those directories. It only looks at the immediate children of <DataDir>/deployments, so plugin caches
// and everything else under DataDir are never touched.
type DeploymentSweeper struct {
	dir     string
	maxAge  time.Duration
	delete  bool
	logger  logging.Logger
	metrics deploymentSweeperMetrics
	now     func() time.Time
}

type deploymentSweeperMetrics struct {
	scanned tally.Gauge
	stale   tally.Counter
	deleted tally.Counter
	errors  tally.Counter
}

// NewDeploymentSweeper returns a sweeper for the deployments directory under dataDir. A zero maxAge uses
// DefaultDeploymentMaxAge. Unless deleteStale is true the sweeper only logs what it would delete.
func NewDeploymentSweeper(dataDir string, maxAge time.Duration, deleteStale bool, logger logging.Logger, scope tally.Scope) (*DeploymentSweeper, error) {
	if dataDir == "" || filepath.Clean(dataDir) == string(filepath.Separator) {
		return nil, errors.Errorf("refusing to sweep deployments under data dir %q", dataDir)
	}
	if maxAge == 0 {
		maxAge = DefaultDeploymentMaxAge
	}
	if maxAge < MinDeploymentMaxAge {
		return nil, errors.Errorf("deployment sweeper max age %s is below the minimum of %s", maxAge, MinDeploymentMaxAge)
	}

	scope = scope.SubScope("deployment_sweeper")
	return &DeploymentSweeper{
		dir:    filepath.Join(dataDir, deploymentsDirName),
		maxAge: maxAge,
		delete: deleteStale,
		logger: logger,
		metrics: deploymentSweeperMetrics{
			scanned: scope.Gauge("scanned"),
			stale:   scope.Counter("stale"),
			deleted: scope.Counter("deleted"),
			errors:  scope.Counter("errors"),
		},
		now: time.Now,
	}, nil
}

// Run does one sweep. Failing to delete one directory does not stop the sweep.
func (s *DeploymentSweeper) Run(ctx context.Context) error {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		// nothing has been fetched on this worker yet
		return nil
	}
	if err != nil {
		s.metrics.errors.Inc(1)
		return errors.Wrapf(err, "reading %s", s.dir)
	}

	var scanned, stale, deleted, failed int
	for _, entry := range entries {
		if ctx.Err() != nil {
			s.logger.WarnContext(ctx, "deployment sweep interrupted")
			break
		}

		// only real directories: symlinks and stray files are left alone
		if !entry.Type().IsDir() {
			continue
		}
		scanned++

		info, err := entry.Info()
		if err != nil {
			// the directory was removed while we were scanning
			if !os.IsNotExist(err) {
				failed++
				s.logger.WarnContext(ctx, "unable to stat deployment directory", map[string]interface{}{
					"path":  filepath.Join(s.dir, entry.Name()),
					"error": err.Error(),
				})
			}
			continue
		}

		age := s.now().Sub(info.ModTime())
		if age <= s.maxAge {
			continue
		}
		stale++

		path := filepath.Join(s.dir, entry.Name())
		fields := map[string]interface{}{
			"path":    path,
			"age":     age.Round(time.Hour).String(),
			"dry_run": !s.delete,
		}
		if !s.delete {
			s.logger.InfoContext(ctx, "stale deployment directory found", fields)
			continue
		}

		if err := os.RemoveAll(path); err != nil {
			failed++
			fields["error"] = err.Error()
			s.logger.ErrorContext(ctx, "unable to delete stale deployment directory", fields)
			continue
		}
		deleted++
		s.logger.InfoContext(ctx, "deleted stale deployment directory", fields)
	}

	s.metrics.scanned.Update(float64(scanned))
	s.metrics.stale.Inc(int64(stale))
	s.metrics.deleted.Inc(int64(deleted))
	s.metrics.errors.Inc(int64(failed))

	s.logger.InfoContext(ctx, fmt.Sprintf("deployment sweep finished: %d directories, %d stale, %d deleted, %d errors (dry run: %t)",
		scanned, stale, deleted, failed, !s.delete))
	return nil
}
