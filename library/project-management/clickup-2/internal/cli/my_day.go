// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelMyDayCmd(flags *rootFlags) *cobra.Command {
	var flagDue string
	var flagAssignee string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "my-day",
		Short: "Your open tasks across every list, sorted by due date with overdue and stuck flags",
		Long: `Reads the local store (no network) and returns open tasks, sorted by due
date soonest-first, flagging items that are overdue or have no due date.

Filter to a person with --assignee (a username, numeric id, or "me" when a
ClickUp user record has been synced). Restrict to a due window with --due
(e.g. 7d, 24h, 2w). Run 'clickup-2-pp-cli sync' first to populate the store.`,
		Example: `  # Everything open, due soonest first
  clickup-2-pp-cli my-day

  # My tasks due within a week, as JSON
  clickup-2-pp-cli my-day --assignee me --due 7d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("clickup-2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clickup-2-pp-cli sync' first.", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "task", flags.maxAge)

			tasks, err := loadPMTasks(db)
			if err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+noTasksHint)
			}

			var dueWindowMS int64
			if flagDue != "" {
				d, err := parseDurationWindow(flagDue)
				if err != nil {
					return fmt.Errorf("invalid --due %q: %v (use forms like 7d, 24h, 2w)", flagDue, err)
				}
				dueWindowMS = time.Now().Add(d).UnixMilli()
			}

			meID := int64(0)
			if strings.EqualFold(flagAssignee, "me") {
				meID = resolveMeID(db)
			}

			now := time.Now().UnixMilli()
			type row struct {
				ID        string   `json:"id"`
				Name      string   `json:"name"`
				Status    string   `json:"status"`
				Due       string   `json:"due_date,omitempty"`
				Overdue   bool     `json:"overdue"`
				NoDueDate bool     `json:"no_due_date"`
				Assignees []string `json:"assignees,omitempty"`
				URL       string   `json:"url,omitempty"`
			}
			out := []row{}
			for _, t := range tasks {
				if !t.open() {
					continue
				}
				if !t.matchAssignee(flagAssignee, meID) {
					continue
				}
				if dueWindowMS != 0 {
					// keep tasks due before the window end (incl. overdue, incl. no-due)
					if t.DueDate != 0 && t.DueDate > dueWindowMS {
						continue
					}
				}
				names := make([]string, 0, len(t.Assignees))
				for _, a := range t.Assignees {
					if a.Username != "" {
						names = append(names, a.Username)
					}
				}
				out = append(out, row{
					ID:        t.ID,
					Name:      t.Name,
					Status:    t.Status,
					Due:       msToString(t.DueDate),
					Overdue:   t.DueDate != 0 && t.DueDate < now,
					NoDueDate: t.DueDate == 0,
					Assignees: names,
					URL:       t.URL,
				})
			}
			// Sort: due tasks ascending by due, then no-due tasks last.
			sort.SliceStable(out, func(i, j int) bool {
				di, dj := out[i].Due == "", out[j].Due == ""
				if di != dj {
					return !di // tasks with a due date first
				}
				return out[i].Due < out[j].Due
			})

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open tasks match.")
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-12s %-10s %-18s %s\n", "ID", "STATUS", "DUE", "NAME")
			for _, r := range out {
				flag := ""
				if r.Overdue {
					flag = " (overdue)"
				} else if r.NoDueDate {
					flag = " (no due date)"
				}
				due := r.Due
				if due == "" {
					due = "-"
				}
				fmt.Fprintf(w, "%-12s %-10s %-18s %s%s\n", r.ID, r.Status, due, r.Name, flag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDue, "due", "", "Only tasks due within this window (e.g. 7d, 24h, 2w)")
	cmd.Flags().StringVar(&flagAssignee, "assignee", "", "Filter to a username, numeric id, or \"me\"")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// parseDurationWindow accepts Go durations plus d (days) and w (weeks).
func parseDurationWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if strings.HasSuffix(s, "w") {
		n, err := parseLeadingInt(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := parseLeadingInt(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func parseLeadingInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("missing number")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
