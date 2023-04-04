package github

import "fmt"

func BuildRevisionLink(repoName string, revision string) string {
	return fmt.Sprintf("https://github.com/%s/commit/%s", repoName, revision)
}
