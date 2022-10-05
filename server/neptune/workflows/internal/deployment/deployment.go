package deployment

import (
	"fmt"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/root"
)

type Repo struct {
	Owner string
	Name  string
}

func (r *Repo) GetFullName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Name)
}

type Info struct {
	ID         string
	CheckRunID int64
	Revision   string
	Repo       Repo
	Root       root.Root
}
