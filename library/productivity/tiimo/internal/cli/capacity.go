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

// capacityRow is one day's committed-versus-free breakdown.
type capacityRow struct {
	Date           string         `json:"date"`
	Weekday        string         `json:"weekday"`
	Activities     int            `json:"activities"`
	CommittedSecs  int            `json:"committed_seconds"`
	CommittedHuman string         `json:"committed_human"`
	FreeSecs       int            `json:"free_seconds"`
	FreeHuman      string         `json:"free_human"`
	WakingSecs     int            `json:"waking_seconds"`
	UtilizationPct float64        `json:"utilization_pct"`
	ByBucket       map[string]int `json:"by_bucket_seconds"`
}

func newNovelCapacityCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagDays, flagDB string
	var flagDayStart, flagDayEnd string

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Committed versus free minutes per day, broken down by time-of-day bucket.",
		Long: `Total how much of each day is already spoken for.

Tiimo shows you the timeline but never totals it, so "is there room on
Thursday" is a question you can only answer by squinting. This sums activity
durations per day and per time-of-day bucket against a waking window.

All-day activities are counted in the Anytime bucket but excluded from the
committed total, because they do not consume a specific span.`,
		Example: "  tiimo-pp-cli capacity --from 2026-08-14 --to 2026-08-21 --agent",
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
				return writeDryRun(cmd.OutOrStdout(), flags, "capacity")
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

			rows := make([]capacityRow, 0)
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

			waking := (dayEnd - dayStart) * 60
			byDay := map[string][]activityRow{}
			for _, a := range acts {
				byDay[a.Day()] = append(byDay[a.Day()], a)
			}
			for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
				key := day.Format(tiimoDateLayout)
				row := capacityRow{
					Date:       key,
					Weekday:    day.Format("Mon"),
					WakingSecs: waking,
					ByBucket:   map[string]int{},
				}
				for _, a := range byDay[key] {
					row.Activities++
					row.ByBucket[a.Bucket()] += a.Duration
					if !a.IsAllDay {
						row.CommittedSecs += a.Duration
					}
				}
				row.FreeSecs = waking - row.CommittedSecs
				if row.FreeSecs < 0 {
					row.FreeSecs = 0
				}
				if waking > 0 {
					row.UtilizationPct = float64(row.CommittedSecs) / float64(waking) * 100
				}
				row.CommittedHuman = humanDuration(row.CommittedSecs)
				row.FreeHuman = humanDuration(row.FreeSecs)
				rows = append(rows, row)
			}

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(w, "No days in the requested window.")
					return
				}
				buckets := bucketOrder(rows)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				header := "DATE\tDAY\tITEMS\tCOMMITTED\tFREE\tLOAD"
				for _, b := range buckets {
					header += "\t" + b
				}
				fmt.Fprintln(tw, header)
				for _, r := range rows {
					line := fmt.Sprintf("%s\t%s\t%d\t%s\t%s\t%.0f%%",
						r.Date, r.Weekday, r.Activities, r.CommittedHuman, r.FreeHuman, r.UtilizationPct)
					for _, b := range buckets {
						line += "\t" + humanDuration(r.ByBucket[b])
					}
					fmt.Fprintln(tw, line)
				}
				_ = tw.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD); defaults to the start date")
	cmd.Flags().StringVar(&flagDays, "days", "", "Look back this far instead of using --from/--to (e.g. 30, 30d, 4w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().StringVar(&flagDayStart, "day-start", "08:00", "Start of the waking window (HH:MM)")
	cmd.Flags().StringVar(&flagDayEnd, "day-end", "22:00", "End of the waking window (HH:MM)")
	return cmd
}

// bucketOrder returns the buckets actually present, in the app's own order
// rather than alphabetically, with any unrecognized label appended.
func bucketOrder(rows []capacityRow) []string {
	preferred := []string{"Morning", "Afternoon", "Evening", "Anytime"}
	seen := map[string]bool{}
	for _, r := range rows {
		for b := range r.ByBucket {
			seen[b] = true
		}
	}
	out := make([]string, 0, len(seen))
	for _, b := range preferred {
		if seen[b] {
			out = append(out, b)
			delete(seen, b)
		}
	}
	rest := make([]string, 0, len(seen))
	for b := range seen {
		rest = append(rest, b)
	}
	sort.Strings(rest)
	return append(out, rest...)
}
