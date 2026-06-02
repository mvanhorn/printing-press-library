// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli. This is the ClickUp-aware
// stale command: ClickUp's modification field is `date_updated`, a millisecond
// epoch the API returns as a string, which the generic resources-table scanner
// cannot compare correctly. This reads the typed task store instead.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagStatus string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Tasks with no update in N days, optionally filtered by status",
		Long: `Finds open tasks whose date_updated has not moved in at least N days,
read from the local store. Optionally restrict to a status (matched
case-insensitively, e.g. --status review) to find work stuck in one place.

Reads the local store only; run 'clickup-2-pp-cli sync' first.`,
		Example: `  # Open tasks untouched for 14+ days
  clickup-2-pp-cli stale --days 14

  # Tasks stuck in review for a week, as JSON
  clickup-2-pp-cli stale --days 7 --status review --json`,
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

			cutoff := time.Now().AddDate(0, 0, -flagDays).UnixMilli()
			now := time.Now().UnixMilli()

			type row struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Status    string `json:"status"`
				DaysStale int    `json:"days_stale"`
				Updated   string `json:"date_updated,omitempty"`
				URL       string `json:"url,omitempty"`
			}
			out := []row{}
			for _, t := range tasks {
				if !t.open() {
					continue
				}
				if t.DateUpdated == 0 || t.DateUpdated >= cutoff {
					continue
				}
				if flagStatus != "" && !strings.EqualFold(t.Status, flagStatus) {
					continue
				}
				out = append(out, row{
					ID:        t.ID,
					Name:      t.Name,
					Status:    t.Status,
					DaysStale: int((now - t.DateUpdated) / (24 * 3600 * 1000)),
					Updated:   msToString(t.DateUpdated),
					URL:       t.URL,
				})
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].DaysStale > out[j].DaysStale })
			total := len(out)
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				payload := map[string]any{"total": total, "showing": len(out), "days": flagDays, "tasks": out}
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			w := cmd.OutOrStdout()
			if total == 0 {
				fmt.Fprintf(w, "No open tasks stale for %d+ days.\n", flagDays)
				return nil
			}
			fmt.Fprintf(w, "%d stale task(s), no update in %d+ days:\n\n", total, flagDays)
			fmt.Fprintf(w, "%-12s %-10s %-6s %s\n", "ID", "STATUS", "DAYS", "NAME")
			for _, r := range out {
				fmt.Fprintf(w, "%-12s %-10s %-6d %s\n", r.ID, r.Status, r.DaysStale, r.Name)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 14, "Days without an update to consider a task stale")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Only tasks in this status (case-insensitive)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum tasks to show")
	return cmd
}
