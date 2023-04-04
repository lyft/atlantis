package github

import "fmt"

func BuildRevisionURLMarkdown(repoFullName string, revision string) string {
	// uses Markdown formatting to generate the link on GH
	return fmt.Sprintf("[%s](https://github.com/%s/commit/%s)", revision, repoFullName, revision)
}
