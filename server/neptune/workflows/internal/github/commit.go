package github

import "fmt"

type Commit struct {
	Ref    Ref
	Author User
}

type Ref struct {
	Name string
	Type string
}

func (r Ref) String() string {
	// We only support branch type refs
	return fmt.Sprintf("refs/heads/%s", r.Name)
}
