package github

import "fmt"

func BuildRevisionLink(repoFullName string, revision string) string {
	return fmt.Sprintf("https://github.com/%s/commit/%s", repoFullName, revision)
}
