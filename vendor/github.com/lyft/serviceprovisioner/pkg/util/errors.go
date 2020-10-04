package util

import (
	"fmt"
	"strings"
)

type MultiError struct {
	Package   string // package that reported the error
	Operation string // operation that caused the error
	Errors    []error
}

func NewMultiError(pkg, op string, errs []error) error {
	return &MultiError{
		Package:   pkg,
		Operation: op,
		Errors:    errs,
	}
}

func (m *MultiError) Error() string {
	errs := make([]string, len(m.Errors))
	for i, e := range m.Errors {
		errs[i] = e.Error()
	}
	return fmt.Sprintf("%s: found %d errors while %s: %s",
		m.Package, len(errs), m.Operation, strings.Join(errs, "; "))
}
