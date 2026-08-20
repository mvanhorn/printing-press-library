// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Run `tiimo-pp-cli sync` to refresh it.

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// agendaEntry is one activity as rendered by `today` and `agenda`.
type agendaEntry struct {
	Date      string `json:"date"`
	Weekday   string `json:"weekday"`
	Bucket    string `json:"bucket"`
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Title     string `json:"title"`
	Icon      string `json:"icon,omitempty"`
	Done      bool   `json:"done"`
	AllDay    bool   `json:"all_day"`
	External  bool   `json:"external_calendar"`
	Steps     int    `json:"checklist_steps,omitempty"`
	StepsDone int    `json:"checklist_done,omitempty"`
	ID        string `json:"activity_id"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTiimoTodayCmd(flags))
		addNovelCommandIfAbsent(root, newTiimoAgendaCmd(flags))
	})
}

func newTiimoTodayCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagIncludeDone bool

	cmd := &cobra.Command{
		Use:   "today",
		Short: "List today's activities from the local mirror in start-time order (offline read; --include-done keeps completed ones)",
		Long: `Print today's timeline from the local mirror.

This is the read you will run most often, so it works offline and returns
immediately. Run 'tiimo-pp-cli sync' to refresh what it sees.`,
		Example: "  tiimo-pp-cli today",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "today")
			}
			return runAgenda(cmd, flags, "", "", "", flagDB, flagIncludeDone)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&flagIncludeDone, "include-done", true, "Include activities already marked complete")
	return cmd
}

func newTiimoAgendaCmd(flags *rootFlags) *cobra.Command {
	var flagFrom, flagTo, flagDays, flagDB string
	var flagIncludeDone bool

	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "List activities between two dates from the local mirror, filtered by --from/--to or a --days lookback",
		Long: `Print activities between two dates from the local mirror.

Activities carry dozens of fields including nested checklists and recurrence
rules. Combine --agent with --select to keep the payload small enough for an
agent to reason over.`,
		Example: "  tiimo-pp-cli agenda --from 2026-08-14 --to 2026-08-21 --agent --select title,start,duration",
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
				return writeDryRun(cmd.OutOrStdout(), flags, "agenda")
			}
			return runAgenda(cmd, flags, flagFrom, flagTo, flagDays, flagDB, flagIncludeDone)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Window start (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&flagTo, "to", "", "Window end (YYYY-MM-DD); defaults to the start date")
	cmd.Flags().StringVar(&flagDays, "days", "", "Look back this far instead of using --from/--to (e.g. 30, 30d, 4w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&flagIncludeDone, "include-done", true, "Include activities already marked complete")
	return cmd
}

// runAgenda backs both `today` and `agenda`; they differ only in their
// default window.
func runAgenda(cmd *cobra.Command, flags *rootFlags, from, to, days, dbPath string, includeDone bool) error {
	start, end, err := dateWindow(from, to, days)
	if err != nil {
		return err
	}

	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	entries := make([]agendaEntry, 0)
	st, ok, err := openLocalMirror(ctx, cmd, flags, dbPath)
	if err != nil {
		return err
	}
	if !ok {
		return writeNoMirror(cmd, flags, dbPath, entries)
	}
	defer st.Close()

	acts, err := loadActivities(ctx, st.DB(), start, end)
	if err != nil {
		return err
	}

	for _, a := range acts {
		if a.Completed() && !includeDone {
			continue
		}
		e := agendaEntry{
			Date:     a.Day(),
			Bucket:   a.Bucket(),
			Title:    a.Title,
			Icon:     a.IconID,
			Done:     a.Completed(),
			AllDay:   a.IsAllDay,
			External: a.IsReadOnly,
			ID:       a.ActivityID,
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
		if n := len(a.Checklist); n > 0 {
			e.Steps = n
			for _, s := range a.Checklist {
				if s.IsChecked {
					e.StepsDone++
				}
			}
		}
		entries = append(entries, e)
	}

	return writeTiimoResult(cmd, flags, entries, func(w io.Writer) {
		if len(entries) == 0 {
			fmt.Fprintf(w, "Nothing scheduled between %s and %s.\n",
				start.Format(tiimoDateLayout), end.Format(tiimoDateLayout))
			return
		}
		currentDay := ""
		var tw *tabwriter.Writer
		flush := func() {
			if tw != nil {
				_ = tw.Flush()
				tw = nil
			}
		}
		for _, e := range entries {
			if e.Date != currentDay {
				flush()
				if currentDay != "" {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "%s  %s\n", strings.ToUpper(e.Weekday), e.Date)
				currentDay = e.Date
				tw = tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			}
			mark := " "
			if e.Done {
				mark = "x"
			}
			when := e.Start
			if when == "" {
				when = "--:--"
			}
			extras := make([]string, 0, 3)
			if e.Duration != "" {
				extras = append(extras, e.Duration)
			}
			if e.Steps > 0 {
				extras = append(extras, fmt.Sprintf("%d/%d steps", e.StepsDone, e.Steps))
			}
			if e.External {
				extras = append(extras, "calendar")
			}
			fmt.Fprintf(tw, "  [%s]\t%s\t%s\t%s\n", mark, when, e.Title, strings.Join(extras, "  "))
		}
		flush()
	})
}
