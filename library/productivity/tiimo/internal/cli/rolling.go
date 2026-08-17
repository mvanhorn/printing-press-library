// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// rollingEntry is one activity in the rolling view.
type rollingEntry struct {
	Date     string `json:"date"`
	Weekday  string `json:"weekday"`
	Bucket   string `json:"bucket"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Duration string `json:"duration,omitempty"`
	Title    string `json:"title"`
	Icon     string `json:"icon,omitempty"`
	Done     bool   `json:"done"`
	AllDay   bool   `json:"all_day"`
	External bool   `json:"external_calendar"`
}

func newNovelRollingCmd(flags *rootFlags) *cobra.Command {
	var flagDays, flagDB string
	var flagIncludeDone bool

	cmd := &cobra.Command{
		Use:   "rolling",
		Short: "List the next --days days of activities starting from today, grouped by day (rolling window, not Tiimo's Monday-Sunday week)",
		Long: `Show the next N days starting from today.

Tiimo's week view runs Monday to Sunday, so on a Friday the coming week is
almost invisible -- reviewers have called this out as a real gap. A rolling
window always starts from today, so "the next seven days" means the next
seven days.

Activities are grouped by day and ordered by start time within each day.`,
		Example: "  tiimo-pp-cli rolling --days 7",
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
				return writeDryRun(cmd.OutOrStdout(), flags, "rolling")
			}

			days := 7
			if strings.TrimSpace(flagDays) != "" {
				n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(flagDays), "d"))
				if err != nil || n <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --days %q: want a positive whole number of days", flagDays))
				}
				days = n
			}

			// Rolling is forward-looking by definition, so the window runs
			// from today rather than the shared look-back helper.
			from, _, err := dateWindow("", "", "")
			if err != nil {
				return err
			}
			to := from.AddDate(0, 0, days).Add(-1)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries := make([]rollingEntry, 0)
			st, ok, err := openLocalMirror(ctx, cmd, flags, flagDB)
			if err != nil {
				return err
			}
			if !ok {
				return writeNoMirror(cmd, flags, flagDB, entries)
			}
			defer st.Close()

			acts, err := loadActivities(ctx, st.DB(), from, to)
			if err != nil {
				return err
			}

			for _, a := range acts {
				if a.Completed() && !flagIncludeDone {
					continue
				}
				e := rollingEntry{
					Date:     a.Day(),
					Bucket:   a.Bucket(),
					Title:    a.Title,
					Icon:     a.IconID,
					Done:     a.Completed(),
					AllDay:   a.IsAllDay,
					External: a.IsReadOnly,
				}
				if s, ok := a.Start(); ok {
					e.Weekday = s.Format("Mon")
					if !a.IsAllDay {
						e.Start = s.Format("15:04")
					}
				}
				if en, ok := a.End(); ok && !a.IsAllDay {
					e.End = en.Format("15:04")
				}
				if a.Duration > 0 {
					e.Duration = humanDuration(a.Duration)
				}
				entries = append(entries, e)
			}

			return writeTiimoResult(cmd, flags, entries, func(w io.Writer) {
				if len(entries) == 0 {
					fmt.Fprintf(w, "Nothing scheduled in the next %d day(s).\n", days)
					return
				}
				currentDay := ""
				for _, e := range entries {
					if e.Date != currentDay {
						if currentDay != "" {
							fmt.Fprintln(w)
						}
						fmt.Fprintf(w, "%s  %s\n", strings.ToUpper(e.Weekday), e.Date)
						currentDay = e.Date
					}
					when := e.Start
					if when == "" {
						when = "     "
					}
					marker := " "
					if e.Done {
						marker = "x"
					}
					suffix := ""
					if e.Duration != "" {
						suffix = "  " + e.Duration
					}
					if e.External {
						suffix += "  (calendar)"
					}
					fmt.Fprintf(w, "  [%s] %s  %s%s\n", marker, when, e.Title, suffix)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagDays, "days", "7", "Number of days to show, starting today")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&flagIncludeDone, "include-done", false, "Include activities already marked complete")
	return cmd
}
