// Package redact removes credentials from text before it is logged or sent
// outside the process.
package redact

import "regexp"

var (
	// credentialURLPattern matches a credential embedded in a URL, e.g.
	// "https://x-access-token:ghs_xxx@github.com/...".
	credentialURLPattern    = regexp.MustCompile(`://[^/@\s]+@`)
	privateKeyPattern       = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)
	githubTokenPattern      = regexp.MustCompile(`\b(?:gh[opsur]_|github_pat_)[A-Za-z0-9_.-]{20,}`)
	awsAccessKeyIDPattern   = regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)
	bearerTokenPattern      = regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9._~+/-]+=*`)
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b((?:password|passwd|secret|secret[_-]?key|access[_-]?key|api[_-]?key|token)\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
)

// CredentialURLs replaces credentials embedded in URLs with REDACTED.
func CredentialURLs(s string) string {
	return credentialURLPattern.ReplaceAllString(s, "://REDACTED@")
}

// Secrets redacts credentials in URLs plus common secret formats: private
// keys, GitHub tokens, AWS access key IDs, bearer tokens, and password, token,
// and key assignments.
func Secrets(s string) string {
	s = privateKeyPattern.ReplaceAllString(s, "REDACTED PRIVATE KEY")
	s = CredentialURLs(s)
	s = githubTokenPattern.ReplaceAllString(s, "REDACTED")
	s = awsAccessKeyIDPattern.ReplaceAllString(s, "REDACTED")
	s = bearerTokenPattern.ReplaceAllString(s, "${1}REDACTED")
	s = secretAssignmentPattern.ReplaceAllString(s, "${1}REDACTED")
	return s
}
