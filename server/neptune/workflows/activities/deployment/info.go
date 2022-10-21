package deployment

const InfoSchemaVersion = 1.0

type Info struct {
	Version  int
	ID       string
	Revision string
	Repo     Repo
	Root     Root
}

type Repo struct {
	Owner string
	Name  string
}

func (r Repo) GetFullName() string {
	return r.Owner + "/" + r.Name
}

type Root struct {
	Name string
}
