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

// stallRow is one checklist step aggregated across every occurrence of its
// parent activity.
type stallRow struct {
	Activity      string  `json:"activity"`
	Step          string  `json:"step"`
	StepIndex     int     `json:"step_index"`
	Occurrences   int     `json:"occurrences"`
	Checked       int     `json:"checked"`
	CompletionPct float64 `json:"completion_pct"`
	// DropOff is the percentage-point fall from the previous step's
	// completion rate. The largest drop-off is the step where the routine
	// actually breaks down, which is rarely the step with the lowest rate
	// in isolation.
	DropOff float64 `json:"drop_off_pct"`
	IsStall bool    `json:"is_stall_point"`
}

func newNovelStallsCmd(flags *rootFlags) *cobra.Command {
	var flagWeeks, flagDB, flagActivity string
	var flagMinOccurrences int

	cmd := &cobra.Command{
		Use:   "stalls",
		Short: "Find the exact checklist step where a multi-step routine tends to break down.",
		Long: `Locate the step where routines stop getting done.

Tiimo checklists live nested inside their activity -- there is no standalone
checklist endpoint -- so per-step history only exists once you have mirrored
the activities locally. This walks each routine's steps in order and reports
the completion rate per step plus the drop-off from the step before it.

The stall point is the largest drop-off, not simply the least-completed step:
a routine that reliably dies at step 4 will show low rates for steps 4-8, and
the interesting one is step 4.

Use this for step-level failure. Do NOT use it for whole-activity completion
rates; use 'adherence' for that.`,
		Example: "  tiimo-pp-cli stalls --weeks 8",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--weeks=8",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stalls")
			}

			weeks := 8
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

			rows := make([]stallRow, 0)
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
			rows = computeStalls(acts, flagActivity, flagMinOccurrences)

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintf(w, "No checklists with at least %d occurrence(s) in the last %d week(s).\n",
						flagMinOccurrences, weeks)
					fmt.Fprintln(w, "Only activities that carry a checklist can stall.")
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ROUTINE\t#\tSTEP\tDONE\tRATE\tDROP\t")
				currentActivity := ""
				for _, r := range rows {
					name := r.Activity
					if name == currentActivity {
						name = ""
					} else {
						currentActivity = r.Activity
					}
					marker := ""
					if r.IsStall {
						marker = "<- stalls here"
					}
					drop := "-"
					if r.DropOff > 0 {
						drop = fmt.Sprintf("-%.0f%%", r.DropOff)
					}
					fmt.Fprintf(tw, "%s\t%d\t%s\t%d/%d\t%.0f%%\t%s\t%s\n",
						name, r.StepIndex+1, r.Step, r.Checked, r.Occurrences,
						r.CompletionPct*100, drop, marker)
				}
				_ = tw.Flush()
			})
		},
	}

	cmd.Flags().StringVar(&flagWeeks, "weeks", "8", "How many weeks back to analyze")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().StringVar(&flagActivity, "activity", "", "Restrict to one activity title (case-insensitive substring match)")
	cmd.Flags().IntVar(&flagMinOccurrences, "min-occurrences", 2, "Ignore checklists seen fewer times than this")
	return cmd
}

// computeStalls aggregates per-step completion within each activity title.
//
// Steps are keyed by their index rather than their title so that a renamed
// step does not fork into two rows, and the position in the routine -- which
// is what "where does it break" is really asking about -- stays stable.
func computeStalls(acts []activityRow, activityFilter string, minOccurrences int) []stallRow {
	type stepAcc struct {
		title   string
		total   int
		checked int
	}
	// activity title -> step index -> accumulator
	byActivity := map[string]map[int]*stepAcc{}
	occurrences := map[string]int{}
	order := make([]string, 0)

	filter := strings.ToLower(strings.TrimSpace(activityFilter))
	for _, a := range acts {
		if a.IsReadOnly || a.Title == "" || len(a.Checklist) == 0 {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(a.Title), filter) {
			continue
		}
		if _, seen := byActivity[a.Title]; !seen {
			byActivity[a.Title] = map[int]*stepAcc{}
			order = append(order, a.Title)
		}
		occurrences[a.Title]++
		for _, step := range a.Checklist {
			acc, ok := byActivity[a.Title][step.Index]
			if !ok {
				acc = &stepAcc{title: step.Title}
				byActivity[a.Title][step.Index] = acc
			}
			if acc.title == "" {
				acc.title = step.Title
			}
			acc.total++
			if step.IsChecked {
				acc.checked++
			}
		}
	}

	rows := make([]stallRow, 0)
	for _, activity := range order {
		if occurrences[activity] < minOccurrences {
			continue
		}
		steps := byActivity[activity]
		indexes := make([]int, 0, len(steps))
		for idx := range steps {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)

		prevRate := -1.0
		worstDrop := 0.0
		worstAt := -1
		activityRows := make([]stallRow, 0, len(indexes))
		for _, idx := range indexes {
			acc := steps[idx]
			rate := 0.0
			if acc.total > 0 {
				rate = float64(acc.checked) / float64(acc.total)
			}
			row := stallRow{
				Activity:      activity,
				Step:          acc.title,
				StepIndex:     idx,
				Occurrences:   acc.total,
				Checked:       acc.checked,
				CompletionPct: rate,
			}
			if prevRate >= 0 && prevRate > rate {
				row.DropOff = (prevRate - rate) * 100
				if row.DropOff > worstDrop {
					worstDrop = row.DropOff
					worstAt = len(activityRows)
				}
			}
			prevRate = rate
			activityRows = append(activityRows, row)
		}
		// Only flag a stall when there is a real cliff, not ordinary noise.
		if worstAt >= 0 && worstDrop >= 20 {
			activityRows[worstAt].IsStall = true
		}
		rows = append(rows, activityRows...)
	}
	return rows
}
