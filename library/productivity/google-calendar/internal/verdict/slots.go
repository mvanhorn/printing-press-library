// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"sort"
	"time"
)

// MergeIntervals sorts intervals by start and coalesces overlapping or
// touching ranges into a minimal disjoint set. Zero-length and inverted
// intervals are dropped. The input slice is not modified.
func MergeIntervals(in []Interval) []Interval {
	valid := make([]Interval, 0, len(in))
	for _, iv := range in {
		if iv.End.After(iv.Start) {
			valid = append(valid, iv)
		}
	}
	sort.SliceStable(valid, func(i, j int) bool { return valid[i].Start.Before(valid[j].Start) })
	out := make([]Interval, 0, len(valid))
	for _, iv := range valid {
		if len(out) > 0 && !iv.Start.After(out[len(out)-1].End) {
			if iv.End.After(out[len(out)-1].End) {
				out[len(out)-1].End = iv.End
			}
			continue
		}
		out = append(out, iv)
	}
	return out
}

// SubtractBusy returns the free sub-intervals of window not covered by busy.
// busy is merged internally, so callers may pass raw unsorted intervals.
func SubtractBusy(window Interval, busy []Interval) []Interval {
	free := []Interval{}
	if !window.End.After(window.Start) {
		return free
	}
	cursor := window.Start
	for _, b := range MergeIntervals(busy) {
		if !b.End.After(window.Start) || !window.End.After(b.Start) {
			continue // busy block entirely outside the window
		}
		if b.Start.After(cursor) {
			end := minTime(b.Start, window.End)
			if end.After(cursor) {
				free = append(free, Interval{Start: cursor, End: end})
			}
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
		if !cursor.Before(window.End) {
			break
		}
	}
	if cursor.Before(window.End) {
		free = append(free, Interval{Start: cursor, End: window.End})
	}
	return free
}

// FreeWithinWindows computes the free intervals across every window (e.g.
// one 09:00-17:00 window per day) after removing busy time. Windows are
// processed in order; the result preserves that order before ranking.
func FreeWithinWindows(windows, busy []Interval) []Interval {
	free := []Interval{}
	for _, w := range windows {
		free = append(free, SubtractBusy(w, busy)...)
	}
	return free
}

// FindSlots filters free intervals down to those of at least minDur and ranks
// them longest first (ties broken by earlier start). Zero qualifying slots
// returns an empty, non-nil slice — an empty day is an answer, not an error.
func FindSlots(free []Interval, minDur time.Duration) []Interval {
	slots := []Interval{}
	for _, iv := range free {
		if iv.Duration() >= minDur && minDur > 0 {
			slots = append(slots, iv)
		}
	}
	sort.SliceStable(slots, func(i, j int) bool {
		di, dj := slots[i].Duration(), slots[j].Duration()
		if di != dj {
			return di > dj
		}
		return slots[i].Start.Before(slots[j].Start)
	})
	return slots
}

// DailyWindows builds one [startMin, endMin) minutes-of-day window per
// calendar day of loc between from and to, clipped to [from, to]. startMin
// and endMin are minutes after local midnight (e.g. 09:00-17:00 is 540,
// 1020). Days are iterated in loc so DST transitions keep wall-clock
// semantics; the returned intervals are UTC instants.
func DailyWindows(from, to time.Time, startMin, endMin int, loc *time.Location) []Interval {
	windows := []Interval{}
	if !to.After(from) || endMin <= startMin || loc == nil {
		return windows
	}
	localFrom := from.In(loc)
	day := time.Date(localFrom.Year(), localFrom.Month(), localFrom.Day(), 0, 0, 0, 0, loc)
	for !day.After(to.In(loc)) {
		ws := day.Add(time.Duration(startMin) * time.Minute)
		we := day.Add(time.Duration(endMin) * time.Minute)
		if ws.Before(from) {
			ws = from
		}
		if we.After(to) {
			we = to
		}
		if we.After(ws) {
			windows = append(windows, Interval{Start: ws.UTC(), End: we.UTC()})
		}
		day = day.AddDate(0, 0, 1)
	}
	return windows
}
