// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"testing"
	"time"
)

func timedEvent(account, id string, start, end time.Time) Event {
	return Event{
		Account:  account,
		Calendar: account + "-cal",
		ID:       id,
		Summary:  "evt-" + id,
		Start:    start,
		End:      end,
		Status:   "confirmed",
	}
}

func TestIsBusy(t *testing.T) {
	t.Parallel()
	base := timedEvent("a", "e1",
		time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC))

	cases := []struct {
		name   string
		mutate func(Event) Event
		want   bool
	}{
		{"normal confirmed event is busy", func(e Event) Event { return e }, true},
		{"transparent (marked Free) is not busy", func(e Event) Event { e.Transparency = "transparent"; return e }, false},
		{"self-declined is not busy", func(e Event) Event { e.SelfDeclined = true; return e }, false},
		{"cancelled is not busy", func(e Event) Event { e.Status = "cancelled"; return e }, false},
		{"TENTATIVE COUNTS AS BUSY", func(e Event) Event { e.Status = "tentative"; return e }, true},
		{"opaque explicitly is busy", func(e Event) Event { e.Transparency = "opaque"; return e }, true},
		{"cancelled beats opaque transparency", func(e Event) Event { e.Status = "cancelled"; e.Transparency = "opaque"; return e }, false},
		{"all-day without transparency is busy", func(e Event) Event { e.AllDay = true; return e }, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsBusy(tc.mutate(base)); got != tc.want {
				t.Errorf("IsBusy() = %v, want %v", got, tc.want)
			}
		})
	}
}
