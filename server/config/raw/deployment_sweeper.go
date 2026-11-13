package raw

import (
	"time"

	"github.com/pkg/errors"
	"github.com/runatlantis/atlantis/server/config/valid"
)

// DeploymentSweeper configures the temporal worker's cleanup of orphaned deployment directories.
// Leaving it out is safe: the sweeper then only logs what it would delete.
type DeploymentSweeper struct {
	// MaxAge is a Go duration such as "360h". Empty uses the default of 15 days.
	MaxAge string `yaml:"max_age" json:"max_age"`
	// Delete turns on actual deletion. When false the sweeper only logs.
	Delete bool `yaml:"delete" json:"delete"`
}

func (d DeploymentSweeper) Validate() error {
	if d.MaxAge == "" {
		return nil
	}
	if _, err := time.ParseDuration(d.MaxAge); err != nil {
		return errors.Wrap(err, "max_age must be a duration such as 360h")
	}
	return nil
}

func (d DeploymentSweeper) ToValid() valid.DeploymentSweeper {
	// Validate has already checked the format, an empty value stays zero and gets the default.
	maxAge, _ := time.ParseDuration(d.MaxAge)
	return valid.DeploymentSweeper{
		MaxAge: maxAge,
		Delete: d.Delete,
	}
}
