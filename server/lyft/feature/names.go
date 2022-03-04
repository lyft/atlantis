package feature

type Name string

// list of feature names used in the code base. These must be kept in sync with any external config.
const GitHubChecks Name = "github-checks"
const LogPersistence Name = "log-persistence"
