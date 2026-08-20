// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// driftRow is one activity title aggregated across every occurrence in the
// window. Tiimo stores what you planned (startTime, duration) next to what
// actually happened (startTimeActual, durationActual, durationPaused) but
// surfaces almost none of the comparison, which is the whole point here.
type driftRow struct {
	Title             string  `json:"title"`
	Samples           int     `json:"samples"`
	StartedSamples    int     `json:"started_samples"`
	AvgStartDelaySecs int     `json:"avg_start_delay_seconds"`
	AvgPlannedSecs    int     `json:"avg_planned_seconds"`
	AvgActualSecs     int     `json:"avg_actual_seconds"`
	AvgOverrunSecs    int     `json:"avg_overrun_seconds"`
	AvgPausedSecs     int     `json:"avg_paused_seconds"`
	CompletionRate    float64 `json:"completion_rate"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagDays, flagDB string
	var flagLimit int
	var flagSortBy string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show which activities consistently start late, overrun, or spend the most time paused.",
		Long: `Compare what you planned against what actually happened.

Tiimo records an actual start, actual duration, and paused duration alongside
the planned values for every activity, but the app shows you almost none of
it. This aggregates both sides by activity title across a window so the
pattern becomes visible.

Use this command to find chronic overruns and late starts. Do NOT use it to
read a single day's schedule; use 'rolling' for that.`,
		Example: "  tiimo-pp-cli drift --days 30 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--days=30",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}

			switch flagSortBy {
			case "overrun", "delay", "paused":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort-by must be one of overrun, delay, paused"))
			}
			from, to, err := dateWindow(flagFrom, flagTo, flagDays)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]driftRow, 0)
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
			rows = aggregateDrift(acts, flagSortBy, flagLimit)

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintf(w, "No activities with recorded actuals between %s and %s.\n",
						from.Format(tiimoDateLayout), to.Format(tiimoDateLayout))
					fmt.Fprintln(w, "Drift needs activities you actually started, not just scheduled.")
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ACTIVITY\tRUNS\tSTARTED\tLATE BY\tPLANNED\tACTUAL\tOVER BY\tPAUSED\tDONE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%.0f%%\n",
						r.Title, r.Samples, r.StartedSamples,
						signedDuration(r.AvgStartDelaySecs),
						humanDuration(r.AvgPlannedSecs),
						humanDuration(r.AvgActualSecs),
						signedDuration(r.AvgOverrunSecs),
						humanDuration(r.AvgPausedSecs),
						r.CompletionRate*100)
				}
				_ = tw.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD); defaults to the start date")
	cmd.Flags().StringVar(&flagDays, "days", "", "Look back this far instead of using --from/--to (e.g. 30, 30d, 4w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum activity titles to report")
	cmd.Flags().StringVar(&flagSortBy, "sort-by", "overrun", "Rank by: overrun, delay, or paused")
	return cmd
}

// aggregateDrift groups activities by title and averages the planned-versus-
// actual deltas. Occurrences that were never started contribute to the
// completion rate but not to the timing averages, so a habit skipped ten
// times does not read as "always on time".
func aggregateDrift(acts []activityRow, sortBy string, limit int) []driftRow {
	type acc struct {
		samples      int
		started      int
		delaySum     int
		plannedSum   int
		actualSum    int
		overrunSum   int
		pausedSum    int
		completedSum int
	}
	byTitle := map[string]*acc{}
	order := make([]string, 0)

	for _, a := range acts {
		// Imported calendar events are not planned in Tiimo, so their timing
		// says nothing about the user's own drift.
		if a.IsReadOnly || a.Title == "" {
			continue
		}
		e, seen := byTitle[a.Title]
		if !seen {
			e = &acc{}
			byTitle[a.Title] = e
			order = append(order, a.Title)
		}
		e.samples++
		if a.Completed() {
			e.completedSum++
		}

		if !a.Started() {
			// No evidence this occurrence was ever run. Counting it would
			// average real drift against pre-filled defaults and report a
			// confident zero.
			continue
		}
		plannedStart, okPlanned := a.Start()
		actualStart, okActual := parseTiimoTime(a.StartTimeActual)
		hasActualDuration := a.DurationActual > 0

		e.started++
		if okPlanned && okActual {
			e.delaySum += int(actualStart.Sub(plannedStart).Seconds())
		}
		e.plannedSum += a.Duration
		if hasActualDuration {
			e.actualSum += a.DurationActual
			e.overrunSum += a.DurationActual - a.Duration
		}
		e.pausedSum += a.DurationPaused
	}

	rows := make([]driftRow, 0, len(order))
	for _, title := range order {
		e := byTitle[title]
		if e.started == 0 {
			// Nothing was ever actually run, so there is no planned-versus-
			// actual comparison to make. Emitting a row of zeros would read
			// as "always on time" when the truth is "no execution history".
			continue
		}
		row := driftRow{Title: title, Samples: e.samples, StartedSamples: e.started}
		if e.samples > 0 {
			row.CompletionRate = float64(e.completedSum) / float64(e.samples)
		}
		// Divide timing sums by the started count, never the sample count:
		// averaging over occurrences that produced no actuals would silently
		// pull every figure toward zero.
		if e.started > 0 {
			row.AvgStartDelaySecs = e.delaySum / e.started
			row.AvgPlannedSecs = e.plannedSum / e.started
			row.AvgActualSecs = e.actualSum / e.started
			row.AvgOverrunSecs = e.overrunSum / e.started
			row.AvgPausedSecs = e.pausedSum / e.started
		}
		rows = append(rows, row)
	}

	key := func(r driftRow) int {
		switch sortBy {
		case "delay":
			return r.AvgStartDelaySecs
		case "paused":
			return r.AvgPausedSecs
		default:
			return r.AvgOverrunSecs
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if key(rows[i]) != key(rows[j]) {
			return key(rows[i]) > key(rows[j])
		}
		return rows[i].Title < rows[j].Title
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// signedDuration renders a delta that can legitimately be negative, so
// "finished early" and "ran over" stay visually distinct.
func signedDuration(seconds int) string {
	switch {
	case seconds < 0:
		return "-" + humanDuration(-seconds)
	case seconds == 0:
		return "on time"
	default:
		return "+" + humanDuration(seconds)
	}
}
