package root

import (
	"fmt"
)

type Repo struct {
	Owner string
	Name  string
}

func (r *Repo) GetFullName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Name)
}

type DeploymentInfo struct {
	ID         string
	CheckRunID int64
	Revision   string
	Repo       Repo
	Root       Root
}
