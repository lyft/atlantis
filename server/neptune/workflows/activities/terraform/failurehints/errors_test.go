package failurehints_test

import (
	"strings"
	"testing"

	"github.com/runatlantis/atlantis/server/neptune/workflows/activities/terraform/failurehints"
	"github.com/stretchr/testify/assert"
)

func TestErrors_boxedDiagnostics(t *testing.T) {
	log := "Initializing the backend...\n" +
		diagnostic(
			"\x1b[1m\x1b[33mWarning: \x1b[0m\x1b[0m\x1b[1mDeprecated attribute",
			"",
			"The attribute \"acl\" is deprecated.",
		) +
		diagnostic(
			"\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1mUnsupported argument",
			"",
			`  on main.tf line 12, in module "s3":`,
			`  12:   bucket_policy = "x"`,
			"",
			`An argument named "bucket_policy" is not expected here.`,
		) +
		diagnostic("\x1b[1m\x1b[31mError: \x1b[0m\x1b[0m\x1b[1mModule not installed")

	assert.Equal(t, []string{
		"Error: Unsupported argument\n" +
			"\n" +
			`  on main.tf line 12, in module "s3":` + "\n" +
			`  12:   bucket_policy = "x"` + "\n" +
			"\n" +
			`An argument named "bucket_policy" is not expected here.`,
		"Error: Module not installed",
	}, failurehints.Errors(log))
}

func TestErrors_unboxedDiagnostics(t *testing.T) {
	log := "Refreshing state...\n" +
		"\n" +
		"Error: Error acquiring the state lock\n" +
		"\n" +
		"Lock Info:\n" +
		"  ID: 1234\n" +
		"\n" +
		"Error: Unsupported Terraform Core version\n"

	assert.Equal(t, []string{
		"Error: Error acquiring the state lock",
		"Error: Unsupported Terraform Core version",
	}, failurehints.Errors(log))
}

func TestErrors_none(t *testing.T) {
	assert.Empty(t, failurehints.Errors(diagnostic("Warning: Deprecated attribute")))
	assert.Empty(t, failurehints.Errors("exit status 1"))
	assert.Empty(t, failurehints.Errors(""))
}

func TestErrors_limitsCount(t *testing.T) {
	var log strings.Builder
	for i := 0; i < 5; i++ {
		log.WriteString(diagnostic("Error: Module not installed"))
	}
	assert.Len(t, failurehints.Errors(log.String()), 3)
}

func TestErrors_redactsAndTruncates(t *testing.T) {
	log := diagnostic(
		"Error: Failed to download module",
		"",
		"git clone https://x-access-token:ghs_abc123XYZ@github.com/lyft/foo.git failed",
		strings.Repeat("x", 3000),
	)

	errs := failurehints.Errors(log)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "https://REDACTED@github.com/lyft/foo.git")
	assert.NotContains(t, errs[0], "ghs_abc123XYZ")
	assert.Equal(t, 2003, len([]rune(errs[0])))
	assert.True(t, strings.HasSuffix(errs[0], "..."))
}
