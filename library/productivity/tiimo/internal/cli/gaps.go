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

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
)

// gapRow is one unscheduled window on one day.
type gapRow struct {
	Date        string `json:"date"`
	Start       string `json:"start"`
	End         string `json:"end"`
	LengthSecs  int    `json:"length_seconds"`
	LengthHuman string `json:"length_human"`
	AfterTitle  string `json:"after_title,omitempty"`
	BeforeTitle string `json:"before_title,omitempty"`
	// UnscheduledSecs is committed time on this day that has no clock slot
	// (bucket-scheduled activities). It does not reduce the gap, but a caller
	// deciding whether the window is really free needs to know it exists.
	UnscheduledSecs int `json:"unscheduled_committed_seconds,omitempty"`
}

func newNovelGapsCmd(flags *rootFlags) *cobra.Command {
	var flagMin, flagFrom, flagTo, flagDays, flagDB string
	var flagDayStart, flagDayEnd string

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Find the unscheduled windows in a day that are at least a given length.",
		Long: `Show the free windows between scheduled activities.

Tiimo users have asked for a visual representation of the gaps in a day;
this is the queryable version. Gaps are computed as the complement of the
scheduled intervals within the waking window, so overlapping and
back-to-back activities collapse correctly rather than producing phantom
free time.

Use this to fit something new into an existing day.`,
		Example: "  tiimo-pp-cli gaps --min 30m --from 2026-08-14",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--min=30m",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "gaps")
			}

			minLen := 15 * time.Minute
			if flagMin != "" {
				d, err := cliutil.ParseDurationLoose(flagMin)
				if err != nil || d <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --min %q: want a duration like 30m, 1h, or 90m", flagMin))
				}
				minLen = d
			}
			dayStart, err := parseClock(flagDayStart, 8, 0)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --day-start %q: want HH:MM", flagDayStart))
			}
			dayEnd, err := parseClock(flagDayEnd, 22, 0)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --day-end %q: want HH:MM", flagDayEnd))
			}
			if dayEnd <= dayStart {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--day-end must be after --day-start"))
			}

			from, to, err := dateWindow(flagFrom, flagTo, flagDays)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]gapRow, 0)
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
			rows = computeGaps(acts, from, to, minLen, dayStart, dayEnd)

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintf(w, "No free windows of at least %s between %s and %s.\n",
						humanDuration(int(minLen.Seconds())),
						from.Format(tiimoDateLayout), to.Format(tiimoDateLayout))
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "DATE\tFROM\tTO\tFREE\tAFTER\tBEFORE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						r.Date, r.Start, r.End, r.LengthHuman,
						orDash(r.AfterTitle), orDash(r.BeforeTitle))
				}
				_ = tw.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&flagMin, "min", "", "Minimum gap length to report (default 15m)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD); defaults to the start date")
	cmd.Flags().StringVar(&flagDays, "days", "", "Look back this far instead of using --from/--to (e.g. 30, 30d, 4w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().StringVar(&flagDayStart, "day-start", "08:00", "Start of the waking window (HH:MM)")
	cmd.Flags().StringVar(&flagDayEnd, "day-end", "22:00", "End of the waking window (HH:MM)")
	return cmd
}

// interval is a half-open [start,end) span in minutes-from-midnight.
type interval struct {
	start, end int
	title      string
}

// computeGaps returns the free windows per day. All-day activities are
// excluded because they do not occupy a specific span; treating them as
// blocking would wipe out every gap on the day.
func computeGaps(acts []activityRow, from, to time.Time, minLen time.Duration, dayStart, dayEnd int) []gapRow {
	byDay := map[string][]interval{}
	unscheduled := map[string]int{}
	for _, a := range acts {
		// Bucket-scheduled activities have no clock position, so they cannot
		// block a specific window -- but their duration is still committed
		// time. Track it so the caller is told rather than shown a day that
		// looks emptier than it is.
		if !a.ClockScheduled() {
			if !a.IsAllDay && a.Duration > 0 {
				unscheduled[a.Day()] += a.Duration
			}
			continue
		}
		s, okS := a.Start()
		e, okE := a.End()
		if !okS || !okE {
			continue
		}
		day := a.Day()
		startMin := s.Hour()*60 + s.Minute()
		endMin := e.Hour()*60 + e.Minute()
		if e.Day() != s.Day() || endMin > 24*60 {
			// An activity crossing midnight blocks the rest of its start day.
			endMin = 24 * 60
		}
		if endMin <= startMin {
			// Zero-length or malformed span contributes nothing.
			continue
		}
		byDay[day] = append(byDay[day], interval{start: startMin, end: endMin, title: a.Title})
	}

	minMinutes := int(minLen.Minutes())
	rows := make([]gapRow, 0)

	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format(tiimoDateLayout)
		spans := byDay[key]
		sort.Slice(spans, func(i, j int) bool {
			if spans[i].start != spans[j].start {
				return spans[i].start < spans[j].start
			}
			return spans[i].end < spans[j].end
		})

		cursor := dayStart
		var lastTitle string
		emit := func(gapStart, gapEnd int, before string) {
			if gapEnd-gapStart < minMinutes {
				return
			}
			rows = append(rows, gapRow{
				Date:            key,
				Start:           clockString(gapStart),
				End:             clockString(gapEnd),
				LengthSecs:      (gapEnd - gapStart) * 60,
				LengthHuman:     humanDuration((gapEnd - gapStart) * 60),
				AfterTitle:      lastTitle,
				BeforeTitle:     before,
				UnscheduledSecs: unscheduled[key],
			})
		}

		for _, sp := range spans {
			if sp.end <= dayStart || sp.start >= dayEnd {
				continue
			}
			if sp.start > cursor {
				emit(cursor, minInt(sp.start, dayEnd), sp.title)
			}
			// Merge overlaps by only ever moving the cursor forward.
			if sp.end > cursor {
				cursor = sp.end
				lastTitle = sp.title
			}
			if cursor >= dayEnd {
				break
			}
		}
		if cursor < dayEnd {
			emit(cursor, dayEnd, "")
		}
	}
	return rows
}

// parseClock reads an HH:MM flag into minutes-from-midnight, falling back to
// the supplied default when the value is empty.
func parseClock(s string, defH, defM int) (int, error) {
	if s == "" {
		return defH*60 + defM, nil
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

func clockString(minutes int) string {
	if minutes >= 24*60 {
		minutes = 24*60 - 1
	}
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
