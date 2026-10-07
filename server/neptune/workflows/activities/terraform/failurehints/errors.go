package failurehints

import (
	"strings"

	"github.com/runatlantis/atlantis/server/redact"
)

const (
	maxErrors          = 3
	maxErrorBlockRunes = 2000
)

// Errors returns the error diagnostics in a Terraform log, in the order
// Terraform printed them, without color codes and with secrets redacted.
//
// Terraform 0.15+ draws each diagnostic in a box that opens with ╷ and closes
// with ╵, so each box with an "Error:" summary is one error. Older versions
// print no box, so for those each paragraph that starts with "Error:" is one
// error. At most 3 errors are returned, each cut to 2000 characters.
func Errors(log string) []string {
	lines := strings.Split(ansiEscapePattern.ReplaceAllString(log, ""), "\n")

	errs := boxedErrors(lines)
	if len(errs) == 0 {
		errs = unboxedErrors(lines)
	}

	if len(errs) > maxErrors {
		errs = errs[:maxErrors]
	}
	for i, e := range errs {
		errs[i] = truncate(redact.Secrets(e), maxErrorBlockRunes)
	}
	return errs
}

func boxedErrors(lines []string) []string {
	var errs, block []string
	inBox := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(trimmed, "╷"):
			inBox, block = true, nil
		case strings.HasPrefix(trimmed, "╵"):
			if inBox && isError(block) {
				errs = append(errs, strings.TrimSpace(strings.Join(block, "\n")))
			}
			inBox = false
		case inBox:
			block = append(block, strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(trimmed, "│"), " "), " "))
		}
	}
	return errs
}

func unboxedErrors(lines []string) []string {
	var errs, block []string
	flush := func() {
		if isError(block) {
			errs = append(errs, strings.Join(block, "\n"))
		}
		block = nil
	}
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			flush()
			continue
		}
		block = append(block, trimmed)
	}
	flush()
	return errs
}

func isError(block []string) bool {
	for _, l := range block {
		if strings.TrimSpace(l) != "" {
			return strings.HasPrefix(strings.TrimSpace(l), "Error:")
		}
	}
	return false
}

func truncate(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
