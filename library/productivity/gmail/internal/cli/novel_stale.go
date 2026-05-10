package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

// staleResult is the structured output for the stale command.
type staleResult struct {
	ResourceType string  `json:"resource_type"`
	LastSyncedAt string  `json:"last_synced_at"`
	AgeDays      float64 `json:"age_days"`
	TotalSynced  int     `json:"total_synced"`
	HistoryToken string  `json:"history_token,omitempty"`
	Status       string  `json:"status"` // "fresh" | "stale" | "never_synced"
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Check when your local mailbox was last synced and whether the history token is still fresh",
		Long: `Reports the last sync timestamp, history token age, and total synced count
from the local sync_state table. Run this before querying the local store
to verify your data is current.

A history token older than 7 days is marked stale — Gmail expires tokens
after ~7 days of no incremental sync. Run 'gmail-pp-cli sync --full' to
recover from an expired token.`,
		Example: `  gmail-pp-cli stale
  gmail-pp-cli stale --json
  gmail-pp-cli stale --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gmail-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\n\nRun 'gmail-pp-cli sync --full' to create the local store", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT resource_type, COALESCE(last_cursor,''), COALESCE(last_synced_at,''), COALESCE(total_count,0)
				 FROM sync_state ORDER BY last_synced_at DESC`)
			if err != nil {
				return fmt.Errorf("querying sync_state: %w", err)
			}
			defer rows.Close()

			var results []staleResult
			for rows.Next() {
				var rt, cursor, syncedAt string
				var total int
				if err := rows.Scan(&rt, &cursor, &syncedAt, &total); err != nil {
					continue
				}
				r := staleResult{
					ResourceType: rt,
					LastSyncedAt: syncedAt,
					TotalSynced:  total,
					HistoryToken: cursor,
				}
				if syncedAt == "" {
					r.Status = "never_synced"
				} else {
					t, parseErr := time.Parse(time.RFC3339, syncedAt)
					if parseErr != nil {
						t, parseErr = time.Parse("2006-01-02 15:04:05", syncedAt)
					}
					if parseErr == nil {
						r.AgeDays = time.Since(t).Hours() / 24
						if r.AgeDays > 7 {
							r.Status = "stale"
						} else {
							r.Status = "fresh"
						}
					} else {
						r.Status = "unknown"
					}
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading sync_state: %w", err)
			}

			if len(results) == 0 {
				msg := "Local mailbox has never been synced. Run 'gmail-pp-cli sync --full' to populate."
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]string{
						"status":  "never_synced",
						"message": msg,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return notFoundErr(fmt.Errorf("never synced"))
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "RESOURCE\tSTATUS\tLAST SYNCED\tAGE (DAYS)\tTOTAL")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%.1f\t%d\n",
					r.ResourceType, r.Status, r.LastSyncedAt, r.AgeDays, r.TotalSynced)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database (default: ~/.local/share/gmail-pp-cli/data.db)")
	return cmd
}
