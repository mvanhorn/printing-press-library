// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command contact idle: find unassigned contacts with no recent activity.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelContactIdleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var limit int
	var includeAssigned bool

	cmd := &cobra.Command{
		Use:         "idle",
		Short:       "Find unassigned contacts with no recent activity worth working.",
		Long:        "Lists contacts from the local mirror that have no assignee and no message activity within --days (default 7).",
		Example:     "  respondio-pp-cli contact idle --days 7 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contact idle")
			}
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]map[string]any, 0), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			results := make([]map[string]any, 0)
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				if !includeAssigned {
					if _, assigned := c["assignee"].(map[string]any); assigned {
						continue
					}
				}
				lmt, hasLMT := c["last_message_time"].(float64)
				if hasLMT && int64(lmt) >= cutoff {
					continue
				}
				results = append(results, c)
				if limit > 0 && len(results) >= limit {
					break
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, c := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%v %v %v\n", c["id"], c["firstName"], c["email"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "idle window in days")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum contacts to return")
	cmd.Flags().BoolVar(&includeAssigned, "assigned", false, "include assigned contacts")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
