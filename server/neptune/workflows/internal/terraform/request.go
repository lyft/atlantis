package terraform

import (
	"github.com/runatlantis/atlantis/server/core/config/valid"
	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/github"
)

type Request struct {
	Repo      github.Repo
	Revision  string
	GlobalCfg valid.GlobalCfg
}
