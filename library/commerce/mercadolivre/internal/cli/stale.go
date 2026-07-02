// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: list products whose newest price snapshot is stale.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string

	cmd := &cobra.Command{
		Use:   "stale [--older-than <duration>]",
		Short: "List products whose newest price snapshot is older than a threshold, so a cotacao is never built on cold prices",
		Long: "Scan locally-stored products and report any whose most recent price snapshot is older " +
			"than the threshold (default 7d), plus products that have no snapshot at all. Each row " +
			"carries the snapshot age so a quotation is never built on cold prices.",
		Example:     "  mercadolivre-pp-cli stale --older-than 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			olderThan := "7d"
			if strings.TrimSpace(flagOlderThan) != "" {
				olderThan = flagOlderThan
			}
			dur, err := cliutil.ParseDurationLoose(olderThan)
			if err != nil {
				return usageErr(fmt.Errorf("--older-than %q: %w", olderThan, err))
			}
			now := time.Now()
			cutoff := now.Add(-dur)

			s, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			products, err := s.ProductsWithLatestSnapshot()
			if err != nil {
				return err
			}

			type staleRow struct {
				CatalogID   string `json:"catalog_id"`
				Name        string `json:"name"`
				LastPrice   string `json:"last_price_at"`
				AgeHours    int64  `json:"age_hours"`
				Reason      string `json:"reason"`
				HasSnapshot bool   `json:"has_snapshot"`
			}
			rows := make([]staleRow, 0)
			for _, p := range products {
				if p.HasSnapshot && p.LastCaptured.After(cutoff) {
					continue // fresh enough
				}
				row := staleRow{
					CatalogID:   p.CatalogID,
					Name:        p.Name,
					HasSnapshot: p.HasSnapshot,
				}
				if p.HasSnapshot {
					row.LastPrice = p.LastCaptured.UTC().Format(time.RFC3339)
					row.AgeHours = int64(now.Sub(p.LastCaptured).Hours())
					row.Reason = "snapshot older than threshold"
				} else {
					row.AgeHours = -1
					row.Reason = "no price snapshot recorded"
				}
				rows = append(rows, row)
			}

			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no stale products (threshold %s)\n", olderThan)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "7d", "Staleness threshold (e.g. 7d, 48h, 1w)")
	return cmd
}
