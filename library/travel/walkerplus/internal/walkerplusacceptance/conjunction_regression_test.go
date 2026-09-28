package walkerplusacceptance_test

import (
	"context"
	"testing"
)

func TestReviewWeeklyConjoinedClosuresKeepNegativeEvidence(t *testing.T) {
	// 2026-10-11..16 runs Sunday through Friday. An event's explicit closed
	// Mondays and Thursdays must not be mistaken for Monday recurrence.
	tests := []struct {
		name, rule          string
		confirmed, possible []string
	}{
		{
			name:      "closure_only",
			rule:      "毎週月曜日と木曜日は休み",
			confirmed: []string{},
			possible:  []string{"2026-10-11", "2026-10-13", "2026-10-14", "2026-10-16"},
		},
		{
			name:      "daily_with_closures",
			rule:      "期間中は毎日開催。毎週月曜日と木曜日は休み",
			confirmed: []string{"2026-10-11", "2026-10-13", "2026-10-14", "2026-10-16"},
			possible:  []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture("ar0313e14001", "月曜と木曜に休む体験会", "2026-10-10", "2026-10-18", "2026年10月10日(土)～10月18日(日)")
			e.schedule = e.period + " " + tc.rule
			s := fixtureSource(t, []sourceEvent{e})
			setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/10/ar0313/eg0120/")
			r, err := fixtureClient(t, s).Shortlist(context.Background(), trip("2026-10-11", "2026-10-16"))
			if err != nil {
				t.Fatal(err)
			}
			got := onlyEvent(t, r)
			if got.Match == nil {
				t.Fatal("source-positive event has no derived match")
			}
			assertStrings(t, "confirmed_days", got.Match.ConfirmedDays, tc.confirmed)
			assertStrings(t, "possible_days", got.Match.PossibleDays, tc.possible)
			assertStrings(t, "closed_weekdays", got.Schedule.ClosedWeekdays, []string{"月曜", "木曜"})
			if got.Schedule.Raw == nil || *got.Schedule.Raw != e.schedule {
				t.Errorf("source schedule was lost: %+v", got.Schedule)
			}
			assertEvidence(t, got, "schedule", tc.rule)
			if r.Coverage.ScannedPages != 1 || r.Coverage.DetailCount != 1 || r.Coverage.RequestCount != 4 {
				t.Errorf("weekly conjunction altered fetch bounds: %+v", r.Coverage)
			}
		})
	}
}
