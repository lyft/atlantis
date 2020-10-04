package manifest

import (
	"path"
	"sort"
	"strings"
)

// A Glob is a glob pattern for matching S3 objects.  The path matching is done
// with path.Match() therefore the negation syntax is '[^' and not '[!', which
// is used by the 'aws s3' CLI.
type Glob string

func (g Glob) String() string { return string(g) }

func (g Glob) Match(name string) bool {
	// WARN: `aws s3` negates with '[!' and Go negates with '[^'
	// we need to rewrite the pattern
	if g == "*" {
		return true
	}
	match, _ := path.Match(g.String(), path.Base(name))
	return match
}

type GlobSet []Glob

func (s GlobSet) String() string {
	a := make([]string, len(s))
	for i, g := range s {
		a[i] = g.String()
	}
	sort.Strings(a)
	return "[" + strings.Join(a, ", ") + "]"
}

func (s GlobSet) Match(name string) bool {
	for _, g := range s {
		if g.Match(name) {
			return true
		}
	}
	return false
}

type FilterSet struct {
	Exclude GlobSet `yaml:"exclude,omitempty" json:"exclude,omitempty" mapstructure:"exclude"`
	Include GlobSet `yaml:"include,omitempty" json:"include,omitempty" mapstructure:"include"`
}

// Match matches name against the filter set.  A match occurs if any Include
// filter returns true or every Exclude filter returns false.
func (s FilterSet) Match(name string) bool {
	return s.Include.Match(name) || !s.Exclude.Match(name)
}
