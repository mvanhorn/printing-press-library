// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// Package cli: deals.go is the flagship compound-query command. It joins the
// `--store`, `--category`, `--since`, `--min-thumbs`, `--limit`, `--deal-id`,
// and `--latest` filters into a single store.DealFilter and renders the result
// through the standard provenance + printJSONFiltered pipeline. Snapshots
// populate via the watch+digest worker; this command is the read API.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"

	"github.com/spf13/cobra"
)

func newDealsCmd(flags *rootFlags) *cobra.Command {
	var (
		storeName string
		category  string
		since     string
		minThumbs int
		limit     int
		dealID    string
		latest    bool
		dbPath    string
	)

	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Compound query over locally captured deal snapshots",
		Long: `Query the local deal_snapshots table by store, category, freshness
window, thumb threshold, or deal id. Returns the latest snapshot per deal_id
by default — pass --latest=false to see every observation.

Snapshots are produced by the watch+digest workflow; this command does not
hit the live Slickdeals feed.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Hot Costco deals captured in the last day
  slickdeals-pp-cli deals --store costco --since 24h --min-thumbs 50 --json

  # Last 20 tech-category snapshots, name/thumbs/store only
  slickdeals-pp-cli deals --category tech --limit 20 --json --select title,thumbs,merchant

  # Every observation of one deal (no dedupe)
  slickdeals-pp-cli deals --deal-id 19510173 --latest=false --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slickdeals-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			filter := store.DealFilter{
				Store:     storeName,
				Category:  category,
				MinThumbs: minThumbs,
				DealID:    dealID,
				Limit:     limit,
				Latest:    latest,
			}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				filter.Since = ts
			}

			results, err := db.QueryDeals(filter)
			if err != nil {
				return fmt.Errorf("querying deals: %w", err)
			}
			if results == nil {
				// Render as JSON [] not null so consumers can iterate without nil guards.
				results = []store.DealSnapshot{}
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"no snapshots match this filter. populate the local store by running 'slickdeals-pp-cli watch' or the digest workflow before querying.")
			}

			syncedAt := time.Now()
			if len(results) > 0 {
				// Most-recent captured_at on the result set — what the
				// provenance envelope's synced_at means here.
				syncedAt = results[0].CapturedAt
				for _, r := range results {
					if r.CapturedAt.After(syncedAt) {
						syncedAt = r.CapturedAt
					}
				}
			}

			prov := DataProvenance{
				Source:       "local",
				SyncedAt:     &syncedAt,
				ResourceType: "deals",
			}

			raw, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("marshaling deals: %w", err)
			}
			wrapped, err := wrapWithProvenance(raw, prov)
			if err != nil {
				return fmt.Errorf("wrapping provenance: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), wrapped, flags)
		},
	}

	cmd.Flags().StringVar(&storeName, "store", "", "Filter by merchant (case-insensitive exact match)")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category (case-insensitive exact match)")
	cmd.Flags().StringVar(&since, "since", "", "Only snapshots captured within this duration (e.g. 24h, 7d, 1w, 30m)")
	cmd.Flags().IntVar(&minThumbs, "min-thumbs", 0, "Minimum thumb score")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dealID, "deal-id", "", "Filter to a single deal_id")
	cmd.Flags().BoolVar(&latest, "latest", true, "Dedupe to the most-recent snapshot per deal_id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slickdeals-pp-cli/data.db)")

	return cmd
}
