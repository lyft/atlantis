// Package failurehints explains common Terraform failures from their log
// output, so a failed run can say what went wrong without the reader opening
// the full log.
package failurehints

import (
	"regexp"
	"strings"

	"github.com/runatlantis/atlantis/server/redact"
)

const maxExcerptRunes = 300

var ansiEscapePattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// Rule recognizes one class of Terraform failure.
type Rule struct {
	Name        string
	Pattern     *regexp.Regexp
	Explanation string
	Fix         string
}

// Hint explains a recognized Terraform failure.
type Hint struct {
	Rule        string
	Explanation string
	Fix         string
	// Excerpt is the log text that matched, without color codes and with
	// secrets redacted.
	Excerpt string
}

// Match returns a hint for the first rule, in order, that matches the log.
//
// Terraform wraps long diagnostics across lines, prefixes them with box
// drawing characters, and separates a diagnostic's parts with blank lines.
// The log is normalized into paragraphs so a phrase split by a line wrap still
// matches, and the matching paragraph becomes the excerpt.
func Match(log string, rules []Rule) (Hint, bool) {
	paragraphs := normalizedParagraphs(log)
	for _, r := range rules {
		for _, p := range paragraphs {
			if r.Pattern.MatchString(p) {
				return Hint{
					Rule:        r.Name,
					Explanation: r.Explanation,
					Fix:         r.Fix,
					Excerpt:     excerpt(p),
				}, true
			}
		}
	}
	return Hint{}, false
}

// DefaultRules returns rules for failures that apply to any Terraform user.
// Callers may append organization-specific rules or put them first.
func DefaultRules() []Rule {
	return []Rule{
		{
			Name:        "state-lock",
			Pattern:     regexp.MustCompile(`Error acquiring the state lock`),
			Explanation: "Another Terraform run holds the state lock for this root.",
			Fix:         "Wait for the other run to finish and re-run the plan. If no run is active, the lock is stale and can be released with terraform force-unlock using the lock ID shown in the log.",
		},
		{
			Name:        "terraform-version",
			Pattern:     regexp.MustCompile(`Unsupported Terraform Core version`),
			Explanation: "This root runs a Terraform version that the configuration's required_version constraints don't allow.",
			Fix:         "Align the root's Terraform version with the required_version constraints in the root and its modules.",
		},
		{
			Name:        "provider-version-conflict",
			Pattern:     regexp.MustCompile(`no available releases match the given constraints`),
			Explanation: "No single provider version satisfies every version constraint across this root and the modules it uses.",
			Fix:         "Compare the constraints listed in the error. Change the root's required_providers or pin modules to versions whose constraints overlap.",
		},
		{
			Name:        "module-not-installed",
			Pattern:     regexp.MustCompile(`Module not installed`),
			Explanation: "A module the configuration references isn't installed in the working directory.",
			Fix:         "Check the module's source and version. If they are correct, the source may not be reachable from where Terraform runs.",
		},
		{
			Name:        "unsupported-argument",
			Pattern:     regexp.MustCompile(`Unsupported argument|An argument named "[^"]+" is not expected here`),
			Explanation: "The configuration passes an argument that the resource or module doesn't accept.",
			Fix:         "This often follows a provider or module version change. Check the documentation for the version in use and rename or remove the argument.",
		},
		{
			Name:        "access-denied",
			Pattern:     regexp.MustCompile(`\bAccessDenied\b|is not authorized to perform|with an explicit deny`),
			Explanation: "The cloud provider rejected a call made with Terraform's credentials.",
			Fix:         "Check the action and resource named in the error. An explicit deny in a resource-based policy comes from the target resource's own policy, so changing the caller's IAM policy won't fix it.",
		},
	}
}

func normalizedParagraphs(log string) []string {
	var paragraphs, current []string
	flush := func() {
		if len(current) > 0 {
			paragraphs = append(paragraphs, strings.Join(current, " "))
			current = nil
		}
	}
	for _, l := range strings.Split(ansiEscapePattern.ReplaceAllString(log, ""), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "│╷╵"))
		if l == "" {
			flush()
			continue
		}
		current = append(current, l)
	}
	flush()
	return paragraphs
}

func excerpt(paragraph string) string {
	out := redact.Secrets(paragraph)
	if r := []rune(out); len(r) > maxExcerptRunes {
		out = string(r[:maxExcerptRunes]) + "..."
	}
	return out
}
