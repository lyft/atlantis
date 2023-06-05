package temporal_test

import (
	"testing"
	"time"

	"github.com/runatlantis/atlantis/server/neptune/workflows/internal/temporal"
	"gopkg.in/go-playground/assert.v1"
)

func TestNextBusinessDay(t *testing.T) {

	cases := []struct {
		date            time.Time
		nextBusinessDay time.Time
	}{
		// thursday
		{
			date: time.Date(2023, time.June, 1, 10, 0, 0, 0, time.Local),
			nextBusinessDay: time.Date(2023, time.June, 2, 10, 0, 0, 0, time.Local),
		},

		// friday
		{
			date: time.Date(2023, time.June, 2, 10, 0, 0, 0, time.Local),
			nextBusinessDay: time.Date(2023, time.June, 5, 10, 0, 0, 0, time.Local),
		},

		// saturday
		{
			date: time.Date(2023, time.June, 3, 10, 0, 0, 0, time.Local),
			nextBusinessDay: time.Date(2023, time.June, 5, 10, 0, 0, 0, time.Local),
		},
		// sunday
		{
			date: time.Date(2023, time.June, 4, 10, 0, 0, 0, time.Local),
			nextBusinessDay: time.Date(2023, time.June, 5, 10, 0, 0, 0, time.Local),
		},
	}

	for _, c := range cases {
		ca := c
		
		t.Run("", func(t *testing.T) {
			result := temporal.NextBusinessDay(ca.date)
			assert.Equal(t, ca.nextBusinessDay, result)
		})
	}
	
}
