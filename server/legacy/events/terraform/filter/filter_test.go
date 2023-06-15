package filter_test

import (
	"regexp"
	"testing"

	"github.com/runatlantis/atlantis/server/legacy/events/terraform/filter"
	"github.com/stretchr/testify/assert"
)

func TestLogFilter_ShouldFilter(t *testing.T) {
	regex := regexp.MustCompile("abc*")
	filter := filter.LogFilter{
		Regexes: []*regexp.Regexp{regex},
	}
	assert.True(t, filter.ShouldFilterLine("abcd"))
}

func TestLogFilter_ShouldNotFilter(t *testing.T) {
	regex := regexp.MustCompile("abc*")
	filter := filter.LogFilter{
		Regexes: []*regexp.Regexp{regex},
	}
	assert.False(t, filter.ShouldFilterLine("efg"))
}
