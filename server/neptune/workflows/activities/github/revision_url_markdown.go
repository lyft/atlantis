package github

import "fmt"

func BuildRevisionURLMarkdown(repoFullName string, revision string) string {
	// uses Markdown formatting to generate the link on GH
	return fmt.Sprintf("[%s](https://github.com/%s/commit/%s)", revision, repoFullName, revision)
}

func BuildRunURLMarkdown(repoFullName string, revision string, runId int64) string {
	// uses Markdown formatting to generate the link on GH
	return fmt.Sprintf("[%s](https://github.com/%s/runs/%d)", revision, repoFullName, runId)
}
