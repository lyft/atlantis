package github

import "testing"

func TestRedactCredentials(t *testing.T) {
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
			got := redactCredentials(c.in)
			if got != c.want {
				t.Errorf("redactCredentials(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
