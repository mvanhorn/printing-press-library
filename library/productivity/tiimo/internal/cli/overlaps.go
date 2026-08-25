// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// overlapRow is one pair of activities whose scheduled spans intersect.
type overlapRow struct {
	Date             string `json:"date"`
	FirstTitle       string `json:"first_title"`
	FirstStart       string `json:"first_start"`
	FirstEnd         string `json:"first_end"`
	SecondTitle      string `json:"second_title"`
	SecondStart      string `json:"second_start"`
	SecondEnd        string `json:"second_end"`
	OverlapSecs      int    `json:"overlap_seconds"`
	OverlapHuman     string `json:"overlap_human"`
	InvolvesExternal bool   `json:"involves_external_calendar"`
}

func newNovelOverlapsCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagDays, flagDB string
	var flagMin string
	var flagIncludeExternal bool

	cmd := &cobra.Command{
		Use:   "overlaps",
		Short: "List activities that are double-booked against each other.",
		Long: `Find activities whose scheduled spans collide.

Overlap detection is one of the longest-standing requests on Tiimo's own
feedback board and has never shipped. A local interval join answers it
directly.

By default this includes collisions with imported external-calendar events,
because those are exactly the ones a Tiimo-only view hides from you.`,
		Example: "  tiimo-pp-cli overlaps --from 2026-08-14 --to 2026-08-21",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--days=7",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "overlaps")
			}

			minOverlap := time.Duration(0)
			if flagMin != "" {
				d, err := parseLooseMinutes(flagMin)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --min %q: want a duration like 5m", flagMin))
				}
				minOverlap = d
			}

			from, to, err := dateWindow(flagFrom, flagTo, flagDays)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]overlapRow, 0)
			st, ok, err := openLocalMirror(ctx, cmd, flags, flagDB)
			if err != nil {
				return err
			}
			if !ok {
				return writeNoMirror(cmd, flags, flagDB, rows)
			}
			defer st.Close()

			acts, err := loadActivities(ctx, st.DB(), from, to)
			if err != nil {
				return err
			}
			rows = findOverlaps(acts, minOverlap, flagIncludeExternal)

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintf(w, "No overlapping activities between %s and %s.\n",
						from.Format(tiimoDateLayout), to.Format(tiimoDateLayout))
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DATE\tOVERLAP\tACTIVITY A\tSPAN A\tACTIVITY B\tSPAN B")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s-%s\t%s\t%s-%s\n",
						r.Date, r.OverlapHuman,
						r.FirstTitle, r.FirstStart, r.FirstEnd,
						r.SecondTitle, r.SecondStart, r.SecondEnd)
				}
				_ = tw.Flush()
				fmt.Fprintf(w, "\n%d overlapping pair(s).\n", len(rows))
			})
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD); defaults to the start date")
	cmd.Flags().StringVar(&flagDays, "days", "", "Look back this far instead of using --from/--to (e.g. 30, 30d, 4w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().StringVar(&flagMin, "min", "", "Ignore overlaps shorter than this (e.g. 5m)")
	cmd.Flags().BoolVar(&flagIncludeExternal, "include-external", true, "Include collisions with imported external-calendar events")
	return cmd
}

// findOverlaps does a per-day pairwise sweep. Activities are sorted by start,
// so the inner loop can stop as soon as a candidate starts at or after the
// current activity's end.
//
// All-day activities are excluded: they nominally span the whole day and
// would collide with everything, which is noise rather than a finding.
func findOverlaps(acts []activityRow, minOverlap time.Duration, includeExternal bool) []overlapRow {
	type span struct {
		row        activityRow
		start, end time.Time
	}
	byDay := map[string][]span{}
	for _, a := range acts {
		// Only clock-scheduled activities can genuinely collide. Bucket-
		// scheduled ones all sit at midnight, so comparing them pairs every
		// activity with every other.
		if !a.ClockScheduled() {
			continue
		}
		if !includeExternal && a.IsReadOnly {
			continue
		}
		s, okS := a.Start()
		e, okE := a.End()
		if !okS || !okE || !e.After(s) {
			continue
		}
		byDay[a.Day()] = append(byDay[a.Day()], span{row: a, start: s, end: e})
	}

	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)

	rows := make([]overlapRow, 0)
	for _, day := range days {
		spans := byDay[day]
		sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
		for i := 0; i < len(spans); i++ {
			for j := i + 1; j < len(spans); j++ {
				if !spans[j].start.Before(spans[i].end) {
					break // sorted by start, so nothing later can overlap either
				}
				overlapEnd := spans[i].end
				if spans[j].end.Before(overlapEnd) {
					overlapEnd = spans[j].end
				}
				d := overlapEnd.Sub(spans[j].start)
				if d <= 0 || d < minOverlap {
					continue
				}
				rows = append(rows, overlapRow{
					Date:             day,
					FirstTitle:       spans[i].row.Title,
					FirstStart:       spans[i].start.Format("15:04"),
					FirstEnd:         spans[i].end.Format("15:04"),
					SecondTitle:      spans[j].row.Title,
					SecondStart:      spans[j].start.Format("15:04"),
					SecondEnd:        spans[j].end.Format("15:04"),
					OverlapSecs:      int(d.Seconds()),
					OverlapHuman:     humanDuration(int(d.Seconds())),
					InvolvesExternal: spans[i].row.IsReadOnly || spans[j].row.IsReadOnly,
				})
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].OverlapSecs > rows[j].OverlapSecs
	})
	return rows
}

// parseLooseMinutes accepts the same duration spellings as the rest of the
// CLI without pulling the cliutil import into this file's surface twice.
func parseLooseMinutes(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}
	var n int
	if _, scanErr := fmt.Sscanf(s, "%d", &n); scanErr == nil && n >= 0 {
		return time.Duration(n) * time.Minute, nil
	}
	return 0, err
}
