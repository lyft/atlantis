package raw

import (
	"errors"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/runatlantis/atlantis/server/config/valid"
)

const (
	defaultLockSweeperMinAgeMinutes = 120
	// A Terraform activity can run for up to 60 minutes, so a shorter threshold could delete the
	// lock of a run that is still going.
	minLockSweeperMinAgeMinutes = 60
)

// LockSweeper configures the job that clears Terraform state locks orphaned by killed workers.
// It is off unless enabled, and runs in dry-run mode unless dry_run is explicitly false.
type LockSweeper struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	DryRun     *bool  `yaml:"dry_run" json:"dry_run"`
	Bucket     string `yaml:"bucket" json:"bucket"`
	KeyPrefix  string `yaml:"key_prefix" json:"key_prefix"`
	Repository string `yaml:"repository" json:"repository"`
	Table      string `yaml:"table" json:"table"`
	// OwnerPrefix limits automatic deletes to locks whose holder (the lock's Who field) starts with it,
	// so a lock taken by a person running Terraform locally is never deleted. Required when enabled.
	OwnerPrefix   string `yaml:"owner_prefix" json:"owner_prefix"`
	MinAgeMinutes int    `yaml:"min_age_minutes" json:"min_age_minutes"`
}

func (l LockSweeper) Validate() error {
	if !l.Enabled {
		return nil
	}
	return validation.ValidateStruct(&l,
		validation.Field(&l.Bucket, validation.Required),
		validation.Field(&l.KeyPrefix, validation.Required),
		validation.Field(&l.Repository, validation.Required),
		validation.Field(&l.Table, validation.Required),
		validation.Field(&l.OwnerPrefix, validation.Required),
		// 0 means "use the default"
		validation.Field(&l.MinAgeMinutes, validation.By(func(v interface{}) error {
			if n := v.(int); n != 0 && n < minLockSweeperMinAgeMinutes {
				return errors.New("must be at least 60 minutes, the longest a Terraform activity can run")
			}
			return nil
		})),
	)
}

func (l LockSweeper) ToValid() valid.LockSweeper {
	if !l.Enabled {
		return valid.LockSweeper{}
	}
	minAge := l.MinAgeMinutes
	if minAge == 0 {
		minAge = defaultLockSweeperMinAgeMinutes
	}
	dryRun := true
	if l.DryRun != nil {
		dryRun = *l.DryRun
	}
	return valid.LockSweeper{
		Enabled:       l.Enabled,
		DryRun:        dryRun,
		Bucket:        l.Bucket,
		KeyPrefix:     l.KeyPrefix,
		Repository:    l.Repository,
		Table:         l.Table,
		OwnerPrefix:   l.OwnerPrefix,
		MinAgeMinutes: minAge,
	}
}
