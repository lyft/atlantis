package failurehints_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform/failurehints"
	"github.com/stretchr/testify/assert"
)

// diagnostic renders lines the way Terraform prints an error diagnostic in
// color: a box drawn with │, ╷, and ╵ around red and bold text.
func diagnostic(lines ...string) string {
	var b strings.Builder
	b.WriteString("\x1b[31m╷\x1b[0m\x1b[0m\n")
	for _, l := range lines {
		b.WriteString("\x1b[31m│\x1b[0m \x1b[0m" + l + "\x1b[0m\n")
	}
	b.WriteString("\x1b[31m╵\x1b[0m\x1b[0m\n")
	return b.String()
}

func TestMatch_defaultRules(t *testing.T) {
	cases := []struct {
		name        string
		log         string
		wantRule    string
		wantExcerpt string
	}{
		{
			name: "state lock",
			log: diagnostic(
				"\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1mError acquiring the state lock",
				"",
				"Error message: ConditionalCheckFailedException: The conditional request failed",
			),
			wantRule:    "state-lock",
			wantExcerpt: "Error: Error acquiring the state lock",
		},
		{
			name: "provider version conflict wrapped across lines",
			log: diagnostic(
				"\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1mFailed to query available provider packages",
				"",
				"Could not retrieve the list of available versions for provider",
				"hashicorp/aws: no available releases match the given constraints ~> 5.51,",
				">= 6.0.0, >= 6.12.0",
			),
			wantRule:    "provider-version-conflict",
			wantExcerpt: "Could not retrieve the list of available versions for provider hashicorp/aws: no available releases match the given constraints ~> 5.51, >= 6.0.0, >= 6.12.0",
		},
		{
			name: "phrase split by a line wrap",
			log: diagnostic(
				"hashicorp/aws: no available releases match the",
				"given constraints >= 6.0",
			),
			wantRule:    "provider-version-conflict",
			wantExcerpt: "hashicorp/aws: no available releases match the given constraints >= 6.0",
		},
		{
			name:        "terraform version",
			log:         diagnostic("Error: Unsupported Terraform Core version"),
			wantRule:    "terraform-version",
			wantExcerpt: "Error: Unsupported Terraform Core version",
		},
		{
			name:        "module not installed",
			log:         diagnostic("Error: Module not installed", "", `This module is not yet installed. Run "terraform init" to install all modules required by this configuration.`),
			wantRule:    "module-not-installed",
			wantExcerpt: "Error: Module not installed",
		},
		{
			name: "unsupported argument",
			log: diagnostic(
				"Error: Unsupported argument",
				"",
				`on main.tf line 12, in module "s3":`,
				`12:   bucket_policy = "x"`,
				"",
				`An argument named "bucket_policy" is not expected here.`,
			),
			wantRule:    "unsupported-argument",
			wantExcerpt: "Error: Unsupported argument",
		},
		{
			name:        "access denied by resource policy",
			log:         diagnostic("Error: reading S3 Bucket (jobs): api error AccessDenied: User is not authorized to perform: s3:GetObject with an explicit deny in a resource-based policy"),
			wantRule:    "access-denied",
			wantExcerpt: "Error: reading S3 Bucket (jobs): api error AccessDenied: User is not authorized to perform: s3:GetObject with an explicit deny in a resource-based policy",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hint, ok := failurehints.Match(c.log, failurehints.DefaultRules())
			assert.True(t, ok)
			assert.Equal(t, c.wantRule, hint.Rule)
			assert.Equal(t, c.wantExcerpt, hint.Excerpt)
			assert.NotEmpty(t, hint.Explanation)
			assert.NotEmpty(t, hint.Fix)
		})
	}
}

func TestMatch_noMatch(t *testing.T) {
	_, ok := failurehints.Match(diagnostic("Error: something nobody has seen before"), failurehints.DefaultRules())
	assert.False(t, ok)

	_, ok = failurehints.Match("", failurehints.DefaultRules())
	assert.False(t, ok)
}

func TestMatch_firstRuleWins(t *testing.T) {
	log := diagnostic("Error: Error acquiring the state lock") + diagnostic("Error: Unsupported argument")
	rules := []failurehints.Rule{
		{Name: "argument", Pattern: regexp.MustCompile(`Unsupported argument`)},
		{Name: "lock", Pattern: regexp.MustCompile(`state lock`)},
	}

	hint, ok := failurehints.Match(log, rules)
	assert.True(t, ok)
	assert.Equal(t, "argument", hint.Rule)
}

func TestMatch_excerptRedactsSecrets(t *testing.T) {
	log := diagnostic("Error: Error acquiring the state lock while fetching https://x-access-token:ghs_abc123XYZ@github.com/lyft/foo.git")

	hint, ok := failurehints.Match(log, failurehints.DefaultRules())
	assert.True(t, ok)
	assert.Equal(t, "Error: Error acquiring the state lock while fetching https://REDACTED@github.com/lyft/foo.git", hint.Excerpt)
}

func TestMatch_excerptTruncated(t *testing.T) {
	log := diagnostic("Error: Error acquiring the state lock " + strings.Repeat("é", 400))

	hint, ok := failurehints.Match(log, failurehints.DefaultRules())
	assert.True(t, ok)
	assert.Equal(t, 303, len([]rune(hint.Excerpt)))
	assert.True(t, strings.HasSuffix(hint.Excerpt, "..."))
}

func TestDefaultRules_returnsCopy(t *testing.T) {
	rules := failurehints.DefaultRules()
	rules[0].Name = "changed"
	assert.NotEqual(t, "changed", failurehints.DefaultRules()[0].Name)
}
