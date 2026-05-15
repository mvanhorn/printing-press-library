// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

type staleDemandRow struct {
	ID             string  `json:"id"`
	Service        string  `json:"service"`
	Category       string  `json:"category"`
	LastSeenAt     int64   `json:"last_seen_at"`
	HoursSinceLast float64 `json:"hours_since_last_seen"`
}

func newDemandStaleCmd(flags *rootFlags) *cobra.Command {
	var hours int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Demand entries not refreshed by `sync` in the last N hours.",
		Long: `List demand entries whose last_seen_at is older than --hours hours ago. Useful
for finding advertisers that have churned off the production lane or that are
being filtered out by a category gate the local mirror hasn't observed yet.

Reads from the local store. Run 'operon-pp-cli sync' to refresh.`,
		Example: strings.Trim(`
  operon-pp-cli demand stale
  operon-pp-cli demand stale --hours 48
  operon-pp-cli demand stale --hours 12 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would list demand entries with last_seen_at older than %d hours\n", hours)
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			// Empty-store detection: if there is no sync state for demand
			// at all, we tell the user how to populate it before complaining.
			lastSync, rowsSynced, err := st.LastSync(ctx, "demand_entries")
			if err != nil {
				return apiErr(fmt.Errorf("reading sync state: %w", err))
			}
			if lastSync == 0 && rowsSynced == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []staleDemandRow{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No demand entries synced. Run `operon-pp-cli sync` first.")
				return nil
			}

			cutoff := time.Now().Add(-time.Duration(hours) * time.Hour).UnixMilli()
			entries, err := st.ListStaleDemand(ctx, cutoff)
			if err != nil {
				return apiErr(err)
			}

			now := time.Now().UnixMilli()
			out := make([]staleDemandRow, 0, len(entries))
			for _, e := range entries {
				hoursSince := float64(now-e.LastSeenAt) / float64(time.Hour/time.Millisecond)
				out = append(out, staleDemandRow{
					ID:             e.ID,
					Service:        e.Service,
					Category:       e.Category,
					LastSeenAt:     e.LastSeenAt,
					HoursSinceLast: hoursSince,
				})
			}

			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no demand entries stale beyond %d hours\n", hours)
				return nil
			}
			headers := []string{"id", "service", "category", "hours_since_last_seen"}
			rows := make([][]string, 0, len(out))
			for _, r := range out {
				rows = append(rows, []string{r.ID, r.Service, r.Category, fmt.Sprintf("%.1f", r.HoursSinceLast)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&hours, "hours", 24, "Threshold in hours: entries last seen before this many hours ago count as stale")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}
