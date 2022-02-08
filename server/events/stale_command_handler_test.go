package events_test

import (
	"errors"
	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	. "github.com/runatlantis/atlantis/testing"
	"github.com/uber-go/tally"
	"testing"
	"time"
)

func TestStaleCommandHandler_CommandIsStale(t *testing.T) {
	testScope := tally.NewTestScope("test", nil)
	staleCommandHandler := &events.StaleCommandHandler{
		Counter: testScope.Counter("dropped_commands"),
	}
	olderTimestamp := time.Unix(123, 456)
	newerTimestamp := time.Unix(123, 457)
	cases := []struct {
		Description      string
		PullStatus       models.PullStatus
		CommandTimestamp time.Time
		Expected         bool
	}{
		{
			Description: "simple stale command",
			PullStatus: models.PullStatus{
				LastEventTimestamp: newerTimestamp,
			},
			CommandTimestamp: olderTimestamp,
			Expected:         true,
		},
		{
			Description: "simple not stale command",
			PullStatus: models.PullStatus{
				LastEventTimestamp: olderTimestamp,
			},
			CommandTimestamp: newerTimestamp,
			Expected:         false,
		},
	}
	for _, c := range cases {
		t.Run(c.Description, func(t *testing.T) {
			ctx := models.ProjectCommandContext{
				EventTimestamp: c.CommandTimestamp,
			}

			RegisterMockTestingT(t)
			mockPullStatusFetcher := mocks.NewMockPullStatusFetcher()
			When(mockPullStatusFetcher.GetPullStatus(
				matchers.AnyModelsPullRequest(),
			)).ThenReturn(&c.PullStatus, nil)

			Assert(t, c.Expected == staleCommandHandler.CommandIsStale(ctx, mockPullStatusFetcher),
				"CommandIsStale returned value should be %v", c.Expected)
		})
	}
	Assert(t, testScope.Snapshot().Counters()["test.dropped_commands+"].Value() == 1, "counted commands doesn't equal 1")
}

func TestStaleCommandHandler_CommandIsStale_NilPullModel(t *testing.T) {
	testScope := tally.NewTestScope("test", nil)
	staleCommandHandler := &events.StaleCommandHandler{
		Counter: testScope.Counter("dropped_commands"),
	}
	RegisterMockTestingT(t)
	mockPullStatusFetcher := mocks.NewMockPullStatusFetcher()
	When(mockPullStatusFetcher.GetPullStatus(
		matchers.AnyModelsPullRequest(),
	)).ThenReturn(nil, nil)
	Assert(t, staleCommandHandler.CommandIsStale(models.ProjectCommandContext{}, mockPullStatusFetcher) == false,
		"CommandIsStale returned value should be false")
	Assert(t, testScope.Snapshot().Counters()["test.dropped_commands+"].Value() == 0, "counted commands doesn't equal 1")
}

func TestStaleCommandHandler_CommandIsStale_FetchError(t *testing.T) {
	testScope := tally.NewTestScope("test", nil)
	staleCommandHandler := &events.StaleCommandHandler{
		Counter: testScope.Counter("dropped_commands"),
	}
	RegisterMockTestingT(t)
	mockPullStatusFetcher := mocks.NewMockPullStatusFetcher()
	When(mockPullStatusFetcher.GetPullStatus(
		matchers.AnyModelsPullRequest(),
	)).ThenReturn(nil, errors.New("failed to fetch request"))
	Assert(t, staleCommandHandler.CommandIsStale(models.ProjectCommandContext{}, mockPullStatusFetcher) == true,
		"CommandIsStale returned value should be false")
	Assert(t, testScope.Snapshot().Counters()["test.dropped_commands+"].Value() == 1, "counted commands doesn't equal 1")
}
