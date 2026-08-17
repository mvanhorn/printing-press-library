// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// adherenceRow is one recurring activity's completion record over the window.
type adherenceRow struct {
	Title          string  `json:"title"`
	RecurrenceType string  `json:"recurrence_type,omitempty"`
	Occurrences    int     `json:"occurrences"`
	Completed      int     `json:"completed"`
	Missed         int     `json:"missed"`
	CompletionRate float64 `json:"completion_rate"`
	CurrentStreak  int     `json:"current_streak"`
	LongestStreak  int     `json:"longest_streak"`
	LastCompleted  string  `json:"last_completed,omitempty"`
}

func newNovelAdherenceCmd(flags *rootFlags) *cobra.Command {
	var flagWeeks, flagDB string
	var flagAll bool
	var flagMinOccurrences int

	cmd := &cobra.Command{
		Use:   "adherence",
		Short: "Completion rate for each recurring activity over a window.",
		Long: `Measure how reliably each recurring activity actually gets done.

Tiimo will tell you a habit exists on the calendar. It will not tell you that
you complete it 62% of weekdays, or that your streak broke eleven days ago.
That needs history, which is what the local mirror is for.

By default only repeating activities are scored, because a one-off has no
adherence to measure. Pass --all to score everything.

Use this for completion rates over time. Do NOT use it to find which step of
a routine fails; use 'stalls' for that.`,
		Example: "  tiimo-pp-cli adherence --weeks 4 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--weeks=4",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "adherence")
			}

			weeks := 4
			if strings.TrimSpace(flagWeeks) != "" {
				n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(flagWeeks), "w"))
				if err != nil || n <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --weeks %q: want a positive whole number of weeks", flagWeeks))
				}
				weeks = n
			}
			from, to, err := dateWindow("", "", fmt.Sprintf("%dd", weeks*7))
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]adherenceRow, 0)
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
			rows = computeAdherence(acts, flagAll, flagMinOccurrences)

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					scope := "repeating activities"
					if flagAll {
						scope = "activities"
					}
					fmt.Fprintf(w, "No %s with at least %d occurrence(s) in the last %d week(s).\n",
						scope, flagMinOccurrences, weeks)
					if !flagAll {
						fmt.Fprintln(w, "Pass --all to score one-off activities too.")
					}
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ACTIVITY\tPATTERN\tDONE\tMISSED\tRATE\tSTREAK\tBEST\tLAST DONE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%.0f%%\t%d\t%d\t%s\n",
						r.Title, orDash(r.RecurrenceType), r.Completed, r.Missed,
						r.CompletionRate*100, r.CurrentStreak, r.LongestStreak,
						orDash(r.LastCompleted))
				}
				_ = tw.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&flagWeeks, "weeks", "4", "How many weeks back to score")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Score one-off activities as well as repeating ones")
	cmd.Flags().IntVar(&flagMinOccurrences, "min-occurrences", 2, "Ignore activities with fewer occurrences than this")
	return cmd
}

// computeAdherence groups occurrences by title and walks them in date order
// to derive completion rate and streaks.
//
// Streaks are computed over occurrences, not calendar days: a weekday-only
// habit should not have its streak broken by the weekend it was never
// scheduled on.
func computeAdherence(acts []activityRow, includeOneOff bool, minOccurrences int) []adherenceRow {
	type occ struct {
		day  string
		done bool
	}
	byTitle := map[string][]occ{}
	recurrence := map[string]string{}
	order := make([]string, 0)

	for _, a := range acts {
		if a.IsReadOnly || a.Title == "" {
			continue
		}
		if !a.IsRepeating && !includeOneOff {
			continue
		}
		if _, seen := byTitle[a.Title]; !seen {
			order = append(order, a.Title)
		}
		byTitle[a.Title] = append(byTitle[a.Title], occ{day: a.Day(), done: a.Completed()})
		if a.RecurrenceType != "" {
			recurrence[a.Title] = a.RecurrenceType
		}
	}

	rows := make([]adherenceRow, 0, len(order))
	for _, title := range order {
		occs := byTitle[title]
		if len(occs) < minOccurrences {
			continue
		}
		sort.Slice(occs, func(i, j int) bool { return occs[i].day < occs[j].day })

		row := adherenceRow{
			Title:          title,
			RecurrenceType: recurrence[title],
			Occurrences:    len(occs),
		}
		current, longest := 0, 0
		for _, o := range occs {
			if o.done {
				row.Completed++
				current++
				if current > longest {
					longest = current
				}
				if o.day > row.LastCompleted {
					row.LastCompleted = o.day
				}
			} else {
				current = 0
			}
		}
		row.Missed = row.Occurrences - row.Completed
		row.CurrentStreak = current
		row.LongestStreak = longest
		if row.Occurrences > 0 {
			row.CompletionRate = float64(row.Completed) / float64(row.Occurrences)
		}
		rows = append(rows, row)
	}

	// Worst adherence first: the point of the command is to surface what is
	// slipping, not to congratulate what is working.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CompletionRate != rows[j].CompletionRate {
			return rows[i].CompletionRate < rows[j].CompletionRate
		}
		return rows[i].Title < rows[j].Title
	})
	return rows
}
