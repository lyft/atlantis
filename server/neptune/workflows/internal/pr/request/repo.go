package request

type Repo struct {
	// FullName is the owner and repo name separated
	// by a "/"
	FullName      string
	Owner         string
	Name          string
	URL           string
	DefaultBranch string
	Credentials   AppCredentials
}

type AppCredentials struct {
	InstallationToken int64
}

type User struct {
	Name string
}
