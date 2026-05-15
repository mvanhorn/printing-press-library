// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

// syncReport is the structured output for `sync`. Field names are snake_case
// so agents parsing --json get a stable contract.
type syncReport struct {
	Source     string `json:"source"`
	DBPath     string `json:"db_path"`
	Total      int    `json:"total"`
	NewEntries int    `json:"new_entries"`
	Updated    int    `json:"updated"`
	DurationMS int64  `json:"duration_ms"`
	LastSyncMS int64  `json:"last_sync_ms"`
	Notes      string `json:"notes,omitempty"`
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the live /demand index into the local SQLite store.",
		Long: `Fetch the current /demand projection from api.operon.so and upsert each entry
into the local store at ~/.cache/operon-pp-cli/store.db. The store backs the
freshness, replay, watch, auction, and trust-history commands.

Sync is the prerequisite for these read-locally commands:
  - demand stale, demand health
  - placement replay, placement watch
  - auction explain
  - campaign trust-history, campaign group-by-wallet

Sync always pulls live data regardless of --data-source.`,
		Example: strings.Trim(`
  operon-pp-cli sync
  operon-pp-cli sync --json
  operon-pp-cli sync --db /tmp/operon.db
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would open store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would GET /demand and upsert each entry\n")
				fmt.Fprintf(cmd.OutOrStdout(), "would record sync state for table: demand_entries\n")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Sandbox-permissive X-Operon-Client header; same pattern as
			// `demand similar`. Production /demand requires it as a UUID.
			headers := map[string]string{}
			if c.Config != nil {
				if v, ok := c.Config.Headers["X-Operon-Client"]; ok && v != "" {
					headers["X-Operon-Client"] = v
				}
			}
			if headers["X-Operon-Client"] == "" {
				if v := os.Getenv("OPERON_CLIENT_UUID"); v != "" {
					headers["X-Operon-Client"] = v
				}
			}
			if headers["X-Operon-Client"] == "" {
				headers["X-Operon-Client"] = "00000000-0000-4000-8000-000000000001"
			}

			ctx := context.Background()
			start := time.Now()

			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			data, err := c.GetWithHeaders("/demand", nil, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var entries []demandEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				return apiErr(fmt.Errorf("parsing /demand response: %w", err))
			}

			var newCount, updatedCount int
			for _, e := range entries {
				isNew, err := st.UpsertDemandEntry(ctx, store.DemandEntry{
					ID:          e.ID,
					Service:     e.Service,
					ServiceType: e.ServiceType,
					Category:    e.Category,
					Description: e.Description,
					Domain:      e.Domain,
					Assets:      e.Assets,
					Type:        e.Type,
				})
				if err != nil {
					return apiErr(fmt.Errorf("upserting %s: %w", e.ID, err))
				}
				if isNew {
					newCount++
				} else {
					updatedCount++
				}
			}

			if err := st.RecordSync(ctx, "demand_entries", len(entries)); err != nil {
				return apiErr(fmt.Errorf("recording sync state: %w", err))
			}

			report := syncReport{
				Source:     "live",
				DBPath:     path,
				Total:      len(entries),
				NewEntries: newCount,
				Updated:    updatedCount,
				DurationMS: time.Since(start).Milliseconds(),
				LastSyncMS: time.Now().UnixMilli(),
			}

			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "synced %d demand entries (%d new, %d updated) in %dms\n",
				report.Total, report.NewEntries, report.Updated, report.DurationMS)
			fmt.Fprintf(w, "store: %s\n", report.DBPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}
