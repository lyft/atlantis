package redact_test

import (
	"testing"

	"github.com/runatlantis/atlantis/server/redact"
	"github.com/stretchr/testify/assert"
)

func TestCredentialURLs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "github app installation token in clone url",
			in:   "about to clone inside RepoFetcher Fetch with params: repo: {CloneURL:https://x-access-token:ghs_abc123XYZ@github.com/lyft/foo.git}",
			want: "about to clone inside RepoFetcher Fetch with params: repo: {CloneURL:https://REDACTED@github.com/lyft/foo.git}",
		},
		{
			name: "command argv rendered by exec.Cmd.String()",
			in:   `command is /usr/bin/git clone --branch main --single-branch https://x-access-token:ghs_abc123XYZ@github.com/lyft/foo.git /tmp/foo`,
			want: `command is /usr/bin/git clone --branch main --single-branch https://REDACTED@github.com/lyft/foo.git /tmp/foo`,
		},
		{
			name: "no credentials present",
			in:   "cloned repo foo to path /tmp/foo",
			want: "cloned repo foo to path /tmp/foo",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, redact.CredentialURLs(c.in))
		})
	}
}

func TestSecrets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "credential url",
			in:   "fetching https://x-access-token:ghs_abc123XYZ@github.com/lyft/foo.git",
			want: "fetching https://REDACTED@github.com/lyft/foo.git",
		},
		{
			name: "standalone github token",
			in:   "using token ghs_16C7e42F292c6912E7710c838347Ae178B4a for request",
			want: "using token REDACTED for request",
		},
		{
			name: "fine-grained github token",
			in:   "GITHUB_TOKEN github_pat_11ABCDEFG0123456789_abcdefghijklmnopqrstuvwxyz",
			want: "GITHUB_TOKEN REDACTED",
		},
		{
			name: "aws access key id",
			in:   "invalid key AKIAIOSFODNN7EXAMPLE supplied",
			want: "invalid key REDACTED supplied",
		},
		{
			name: "bearer token",
			in:   "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.e30.abc-def_123",
			want: "Authorization: Bearer REDACTED",
		},
		{
			name: "secret assignments",
			in:   `password = "hunter2", api_key: abc123; secret_key=xyz`,
			want: `password = REDACTED, api_key: REDACTED; secret_key=REDACTED`,
		},
		{
			name: "private key block",
			in:   "key:\n-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\ndone",
			want: "key:\nREDACTED PRIVATE KEY\ndone",
		},
		{
			name: "no secrets",
			in:   "Error: Error acquiring the state lock",
			want: "Error: Error acquiring the state lock",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redact.Secrets(c.in)
			assert.Equal(t, c.want, got)
			assert.Equal(t, got, redact.Secrets(got), "redaction should be idempotent")
		})
	}
}
