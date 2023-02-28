package version

const (
	// This version removes an activity that computes env vars from commands and instead opts
	// for lazy loading within each of the following steps.
	LazyLoadEnvVars = "lazy-load-env-vars"

	// This version allows rebasing open PRs for a root once a deploy is complete
	RebaseOpenPRs = "rebase-open-prs"
)
