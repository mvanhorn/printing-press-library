// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"testing"
	"time"
)

// TestSlotsInversion is the canonical acceptance case: busy [10:00-11:00] and
// [13:00-14:00] inside a 09:00-17:00 single-day window with a 90m minimum.
// Free gaps are 09-10 (60m, does NOT qualify), 11-13 (120m), 14-17 (180m).
// Expect exactly [14-17, 11-13] — longest first, exact boundaries.
func TestSlotsInversion(t *testing.T) {
	t.Parallel()
	d := func(h, m int) time.Time { return time.Date(2026, 8, 18, h, m, 0, 0, time.UTC) }
	window := Interval{Start: d(9, 0), End: d(17, 0)}
	busy := []Interval{
		{Start: d(10, 0), End: d(11, 0)},
		{Start: d(13, 0), End: d(14, 0)},
	}

	free := FreeWithinWindows([]Interval{window}, busy)
	wantFree := []Interval{
		{Start: d(9, 0), End: d(10, 0)},
		{Start: d(11, 0), End: d(13, 0)},
		{Start: d(14, 0), End: d(17, 0)},
	}
	if len(free) != len(wantFree) {
		t.Fatalf("free gaps = %d, want %d: %+v", len(free), len(wantFree), free)
	}
	for i := range wantFree {
		if !free[i].Start.Equal(wantFree[i].Start) || !free[i].End.Equal(wantFree[i].End) {
			t.Errorf("free[%d] = [%v, %v), want [%v, %v)", i, free[i].Start, free[i].End, wantFree[i].Start, wantFree[i].End)
		}
	}

	slots := FindSlots(free, 90*time.Minute)
	wantSlots := []Interval{
		{Start: d(14, 0), End: d(17, 0)}, // 180m first
		{Start: d(11, 0), End: d(13, 0)}, // 120m second
	}
	if len(slots) != len(wantSlots) {
		t.Fatalf("qualifying slots = %d, want %d: %+v", len(slots), len(wantSlots), slots)
	}
	for i := range wantSlots {
		if !slots[i].Start.Equal(wantSlots[i].Start) || !slots[i].End.Equal(wantSlots[i].End) {
			t.Errorf("slot[%d] = [%v, %v), want [%v, %v) (longest-first ordering)", i, slots[i].Start, slots[i].End, wantSlots[i].Start, wantSlots[i].End)
		}
	}
	for i := 1; i < len(slots); i++ {
		if slots[i].Duration() > slots[i-1].Duration() {
			t.Errorf("slots not ranked longest-first: %v after %v", slots[i], slots[i-1])
		}
	}
}

func TestSlotsEmptyBusyReturnsWholeWindow(t *testing.T) {
	t.Parallel()
	d := func(h, m int) time.Time { return time.Date(2026, 8, 18, h, m, 0, 0, time.UTC) }
	window := Interval{Start: d(9, 0), End: d(17, 0)}
	free := FreeWithinWindows([]Interval{window}, nil)
	if len(free) != 1 || !free[0].Start.Equal(window.Start) || !free[0].End.Equal(window.End) {
		t.Fatalf("empty busy must return the whole window, got %+v", free)
	}
	slots := FindSlots(free, 90*time.Minute)
	if len(slots) != 1 || !slots[0].Start.Equal(window.Start) || !slots[0].End.Equal(window.End) {
		t.Fatalf("whole window should qualify as one slot, got %+v", slots)
	}
}

func TestSlotsZeroQualifyingReturnsEmptyNotError(t *testing.T) {
	t.Parallel()
	d := func(h, m int) time.Time { return time.Date(2026, 8, 18, h, m, 0, 0, time.UTC) }
	window := Interval{Start: d(9, 0), End: d(10, 0)} // only 60m available
	free := FreeWithinWindows([]Interval{window}, nil)
	slots := FindSlots(free, 90*time.Minute)
	if slots == nil {
		t.Fatalf("zero qualifying slots must return an empty list, not nil")
	}
	if len(slots) != 0 {
		t.Fatalf("no slot >= 90m should qualify in a 60m window, got %+v", slots)
	}
}

func TestMergeIntervals(t *testing.T) {
	t.Parallel()
	d := func(h, m int) time.Time { return time.Date(2026, 8, 18, h, m, 0, 0, time.UTC) }
	cases := []struct {
		name string
		in   []Interval
		want []Interval
	}{
		{
			"overlapping merge",
			[]Interval{{d(10, 0), d(11, 0)}, {d(10, 30), d(12, 0)}},
			[]Interval{{d(10, 0), d(12, 0)}},
		},
		{
			"touching merge (busy 10-11 + 11-12 is one block)",
			[]Interval{{d(10, 0), d(11, 0)}, {d(11, 0), d(12, 0)}},
			[]Interval{{d(10, 0), d(12, 0)}},
		},
		{
			"disjoint stay separate and get sorted",
			[]Interval{{d(13, 0), d(14, 0)}, {d(10, 0), d(11, 0)}},
			[]Interval{{d(10, 0), d(11, 0)}, {d(13, 0), d(14, 0)}},
		},
		{
			"inverted and zero-length dropped",
			[]Interval{{d(11, 0), d(10, 0)}, {d(12, 0), d(12, 0)}},
			[]Interval{},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MergeIntervals(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("MergeIntervals = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if !got[i].Start.Equal(tc.want[i].Start) || !got[i].End.Equal(tc.want[i].End) {
					t.Errorf("interval[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDailyWindows(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	// 09:00-17:00 in UTC across two full days (the 8/20 window clips to zero
	// length at the `to` bound and is dropped).
	windows := DailyWindows(from, to, 9*60, 17*60, time.UTC)
	if len(windows) != 2 {
		t.Fatalf("want 2 daily windows, got %d: %+v", len(windows), windows)
	}
	want0 := Interval{time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC)}
	want1 := Interval{time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 19, 17, 0, 0, 0, time.UTC)}
	if !windows[0].Start.Equal(want0.Start) || !windows[0].End.Equal(want0.End) {
		t.Errorf("windows[0] = %+v, want %+v", windows[0], want0)
	}
	if !windows[1].Start.Equal(want1.Start) || !windows[1].End.Equal(want1.End) {
		t.Errorf("windows[1] = %+v, want %+v", windows[1], want1)
	}

	// Mid-day `from` clips the first window's start.
	fromNoon := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	clipped := DailyWindows(fromNoon, to, 9*60, 17*60, time.UTC)
	if len(clipped) == 0 || !clipped[0].Start.Equal(fromNoon) {
		t.Errorf("first window must clip to from=%v, got %+v", fromNoon, clipped)
	}
}
