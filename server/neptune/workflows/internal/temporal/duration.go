package temporal

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

type NextDay func(time.Time) time.Time

func NextBusinessDay(d time.Time) time.Time {
	d = d.Add(24 * time.Hour)

	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.Add(24 * time.Hour)
	}

	return d
}

func UntilHour(ctx workflow.Context, hour int, next NextDay) time.Duration {
	t := workflow.Now(ctx)
	d := time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, t.Location())

	duration := d.Sub(t)

	// if duration is zero or negative, we know our current time is later so let's just
	// wait for the defined period to elapse
	if duration <= 0 {
		d = next(d)

		return d.Sub(t)
	}

	return duration
}
