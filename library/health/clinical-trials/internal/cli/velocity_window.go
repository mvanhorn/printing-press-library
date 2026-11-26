// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// velocity_window.go holds the pure logic of the `velocity` command: which
// calendar years make up the recent and prior windows, how the sampled trials
// fall into them, and the notes that tell a reader how far to trust the result.
// Keeping it free of cobra and flags lets the tests drive it with a fake client.
package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// velocityWindows are two equal-length runs of COMPLETE calendar years: the
// recent window ends last year and the prior window is the same number of years
// immediately before it. The current year is in neither, because it is only
// partly over and would read as a collapse against a full year. The old rule
// (recent = every year from now-N on, prior = everything older) made the recent
// window N+1 years or more and left the prior window unbounded.
type velocityWindows struct {
	RecentFrom, RecentTo int
	PriorFrom, PriorTo   int
}

// velocityWindowsFor places the windows relative to now. The year is read in UTC
// so the result does not depend on the machine's time zone.
func velocityWindowsFor(now time.Time, windowYears int) velocityWindows {
	last := now.UTC().Year() - 1
	return velocityWindows{
		RecentFrom: last - windowYears + 1,
		RecentTo:   last,
		PriorFrom:  last - 2*windowYears + 1,
		PriorTo:    last - windowYears,
	}
}

// yearSpan renders a window as "2023–2025", or "2025" for a one-year window.
func yearSpan(from, to int) string {
	if from == to {
		return fmt.Sprintf("%d", from)
	}
	return fmt.Sprintf("%d–%d", from, to)
}

// windowTally is how a set of trials falls into the windows.
type windowTally struct {
	RecentTrials, PriorTrials int
	RecentEnroll, PriorEnroll int
	// LaterTrials started after the recent window (the current year or later);
	// OlderTrials started before the prior window. Neither enters the growth rate.
	LaterTrials, OlderTrials int
	ByYear                   []yearBucket
}

// tallyByWindow buckets trials by start year and sums each window. A trial with
// no parseable start date cannot be placed on the timeline and is skipped.
func tallyByWindow(trials []Trial, w velocityWindows) windowTally {
	var t windowTally
	byYear := map[int]*yearBucket{}
	for _, tr := range trials {
		yr := startYear(tr.StartDate)
		if yr == 0 {
			continue
		}
		b, ok := byYear[yr]
		if !ok {
			b = &yearBucket{Year: yr}
			byYear[yr] = b
		}
		b.Trials++
		enroll := tr.Enrollment
		if enroll < 0 {
			enroll = 0
		}
		b.Enrollment += enroll
		switch {
		case yr >= w.RecentFrom && yr <= w.RecentTo:
			t.RecentTrials++
			t.RecentEnroll += enroll
		case yr >= w.PriorFrom && yr <= w.PriorTo:
			t.PriorTrials++
			t.PriorEnroll += enroll
		case yr > w.RecentTo:
			t.LaterTrials++
		default:
			t.OlderTrials++
		}
	}
	t.ByYear = make([]yearBucket, 0, len(byYear))
	for _, b := range byYear {
		t.ByYear = append(t.ByYear, *b)
	}
	sort.Slice(t.ByYear, func(i, j int) bool { return t.ByYear[i].Year < t.ByYear[j].Year })
	return t
}

// velocityNotes explains what the numbers rest on. registryTotal is 0 when the
// registry's own count is unknown, in which case no sample note is made.
func velocityNotes(sampled, registryTotal int, w velocityWindows, t windowTally) string {
	var notes []string
	switch {
	case sampled == 0:
		notes = append(notes, "no trials matched; try a broader area term")
	case len(t.ByYear) == 0:
		notes = append(notes, "matched trials have no parseable start dates; cannot place them on a timeline")
	case t.PriorTrials == 0:
		notes = append(notes, fmt.Sprintf("no trials in the prior window (%s); growth reflects newly-appearing activity. Widen --window-years or --sample for a baseline.", yearSpan(w.PriorFrom, w.PriorTo)))
	}
	if sampled > 0 && registryTotal > sampled {
		notes = append(notes, fmt.Sprintf("sample covers %d of %d trials matching this area; the per-year counts and growth come from the sample, not the full registry. Raise --sample and --max-scan-pages for a closer match.", sampled, registryTotal))
	}
	if t.LaterTrials > 0 {
		notes = append(notes, fmt.Sprintf("%d trial(s) started in %d or later (the current year is incomplete) appear in by_year but not in the growth windows.", t.LaterTrials, w.RecentTo+1))
	}
	return strings.Join(notes, " ")
}

// buildVelocityView samples trials for an area and turns them into the view.
// The registry's total is a best-effort extra request: if it fails the view is
// still returned, just without the sample note.
func buildVelocityView(ctx context.Context, c ctgovClient, term string, sample, maxScanPages, windowYears int, now time.Time) (velocityView, error) {
	params := ctgovParams("cond", term)
	trials, err := ctgovFetch(ctx, c, params, sample, maxScanPages)
	if err != nil {
		return velocityView{}, err
	}
	registryTotal := 0
	if len(trials) > 0 {
		if n, cerr := ctgovCount(ctx, c, params); cerr == nil {
			registryTotal = n
		}
	}
	w := velocityWindowsFor(now, windowYears)
	t := tallyByWindow(trials, w)
	return velocityView{
		Query:           term,
		SampleSize:      len(trials),
		RegistryTotal:   registryTotal,
		WindowYears:     windowYears,
		RecentFrom:      w.RecentFrom,
		RecentTo:        w.RecentTo,
		PriorFrom:       w.PriorFrom,
		PriorTo:         w.PriorTo,
		RecentTrials:    t.RecentTrials,
		PriorTrials:     t.PriorTrials,
		TrialGrowthPct:  growthPct(t.RecentTrials, t.PriorTrials),
		RecentEnroll:    t.RecentEnroll,
		PriorEnroll:     t.PriorEnroll,
		EnrollGrowthPct: growthPct(t.RecentEnroll, t.PriorEnroll),
		LaterTrials:     t.LaterTrials,
		OlderTrials:     t.OlderTrials,
		ByYear:          t.ByYear,
		Note:            velocityNotes(len(trials), registryTotal, w, t),
	}, nil
}

// writeVelocityHuman prints the view for a terminal. It takes a plain writer so
// the output can be tested without a cobra command.
func writeVelocityHuman(w io.Writer, v velocityView) error {
	if v.RegistryTotal > v.SampleSize {
		fmt.Fprintf(w, "Trial velocity — %s (sample %d of %d)\n", v.Query, v.SampleSize, v.RegistryTotal)
	} else {
		fmt.Fprintf(w, "Trial velocity — %s (sample %d)\n", v.Query, v.SampleSize)
	}
	fmt.Fprintf(w, "  Recent %s: %d trials (%+d%%), %d enrolled (%+d%%)  vs prior %s: %d trials, %d enrolled\n\n",
		yearSpan(v.RecentFrom, v.RecentTo), v.RecentTrials, v.TrialGrowthPct, v.RecentEnroll, v.EnrollGrowthPct,
		yearSpan(v.PriorFrom, v.PriorTo), v.PriorTrials, v.PriorEnroll)
	if len(v.ByYear) > 0 {
		fmt.Fprintln(w, "By start year:")
		for _, b := range v.ByYear {
			fmt.Fprintf(w, "  %d  %4d trials  %7d enrolled\n", b.Year, b.Trials, b.Enrollment)
		}
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
