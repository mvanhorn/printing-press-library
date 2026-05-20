// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/store"
	"github.com/spf13/cobra"
)

func newSyncStatusCmd(flags *rootFlags) *cobra.Command {
	var flagDBPath string

	cmd := &cobra.Command{
		Use:         "sync-status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Report the age, row counts, and freshness of every synced SQLite table.",
		Long: `Shows the sync state for every resource in the local database: last sync
time, row count, and freshness rating. Useful for automation scripts that
need to check stale data before acting.`,
		Example: "  diretta-pp-cli sync-status\n  diretta-pp-cli sync-status --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = defaultDBPath("diretta-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"status":  "no_data",
						"message": "No local database found. Run 'diretta-pp-cli sync' first.",
					}, flags)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "No local database found. Run 'diretta-pp-cli sync' first.")
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			resources := knownSyncResourceNames()
			type resourceStatus struct {
				Resource   string     `json:"resource"`
				RowCount   int        `json:"row_count"`
				LastSynced *time.Time `json:"last_synced,omitempty"`
				AgeMinutes float64    `json:"age_minutes,omitempty"`
				Freshness  string     `json:"freshness"`
			}

			var statuses []resourceStatus
			now := time.Now()
			for _, resource := range resources {
				_, lastSynced, rowCount, err := db.GetSyncState(resource)
				if err != nil {
					continue
				}
				rs := resourceStatus{
					Resource: resource,
					RowCount: rowCount,
				}
				if !lastSynced.IsZero() {
					t := lastSynced
					rs.LastSynced = &t
					age := now.Sub(lastSynced)
					rs.AgeMinutes = age.Minutes()
					switch {
					case age < 5*time.Minute:
						rs.Freshness = "fresh"
					case age < time.Hour:
						rs.Freshness = "ok"
					case age < 24*time.Hour:
						rs.Freshness = "stale"
					default:
						rs.Freshness = "very_stale"
					}
				} else {
					rs.Freshness = "never_synced"
				}
				statuses = append(statuses, rs)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet) {
				data, _ := json.Marshal(statuses)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(data), true)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "RESOURCE\tROWS\tLAST SYNCED\tFRESHNESS")
			for _, rs := range statuses {
				age := "never"
				if rs.LastSynced != nil {
					age = fmt.Sprintf("%.0fm ago", rs.AgeMinutes)
					if rs.AgeMinutes >= 60 {
						age = fmt.Sprintf("%.1fh ago", rs.AgeMinutes/60)
					}
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", rs.Resource, rs.RowCount, age, rs.Freshness)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Database path (default: ~/.local/share/diretta-pp-cli/data.db)")
	return cmd
}
