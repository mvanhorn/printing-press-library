// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: inspect and manage the local supplier-rating cache.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"

	"github.com/spf13/cobra"
)

// ratingCacheTTLDays mirrors ratingCacheTTL for human-facing copy.
const ratingCacheTTLDays = 14

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cache",
		Short:       "Inspect and manage the local supplier-rating cache",
		Long:        fmt.Sprintf("The tool caches per-airport supplier ratings locally so companies whose rating a given search omits still show a score. Entries expire after %d days and stale ones are purged automatically. Use these subcommands to see what's cached and how fresh it is, or to clear it and force a live refetch on the next search.", ratingCacheTTLDays),
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newCacheStatusCmd(flags))
	cmd.AddCommand(newCacheClearCmd(flags))
	return cmd
}

type cacheStatusRow struct {
	Airport   string `json:"airport"`
	Suppliers int    `json:"suppliers"`
	AgeDays   int    `json:"oldest_age_days"`
	Stale     bool   `json:"stale"`
	NewestAge int    `json:"newest_age_days"`
}

func newCacheStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show cached supplier ratings per airport with their freshness",
		Example:     "  rentalcarspain-pp-cli cache status --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			stats, err := db.SupplierRatingCacheStats(ctx)
			if err != nil {
				return apiErr(err)
			}
			now := time.Now()
			rows := make([]cacheStatusRow, 0, len(stats))
			for _, s := range stats {
				oldestDays := int(now.Sub(s.Oldest).Hours() / 24)
				rows = append(rows, cacheStatusRow{
					Airport: s.Airport, Suppliers: s.Suppliers,
					AgeDays: oldestDays, Stale: oldestDays >= ratingCacheTTLDays,
					NewestAge: int(now.Sub(s.Newest).Hours() / 24),
				})
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"ttl_days": ratingCacheTTLDays, "airports": rows})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "Rating cache is empty. Run a search (suppliers/recommend) to populate it.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "AIRPORT\tSUPPLIERS\tOLDEST\tFRESHNESS")
			for _, r := range rows {
				fresh := "fresh"
				if r.Stale {
					fresh = "STALE (expires on next search)"
				}
				fmt.Fprintf(tw, "%s\t%d\t%dd ago\t%s\n", r.Airport, r.Suppliers, r.AgeDays, fresh)
			}
			tw.Flush()
			fmt.Fprintf(w, "\nRatings expire after %d days; stale entries are purged automatically on the next search for that airport.\n", ratingCacheTTLDays)
			return nil
		},
	}
}

func newCacheClearCmd(flags *rootFlags) *cobra.Command {
	var airport string
	var staleOnly bool
	cmd := &cobra.Command{
		Use:         "clear",
		Short:       "Clear cached supplier ratings (all, one airport, or only stale entries)",
		Example:     "  rentalcarspain-pp-cli cache clear --airport BIO\n  rentalcarspain-pp-cli cache clear --stale-only",
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			var removed int
			if staleOnly {
				removed, err = db.PurgeStaleSupplierRatings(ctx, ratingCacheTTL)
			} else {
				removed, err = db.ClearSupplierRatings(ctx, airport)
			}
			if err != nil {
				return apiErr(err)
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"removed": removed, "airport": airport, "stale_only": staleOnly})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			scope := "all airports"
			if staleOnly {
				scope = "stale entries"
			} else if airport != "" {
				scope = "airport " + airport
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cleared %d cached rating(s) (%s). They will be refetched live on the next search.\n", removed, scope)
			return nil
		},
	}
	cmd.Flags().StringVar(&airport, "airport", "", "Clear only this airport (IATA code); default clears all")
	cmd.Flags().BoolVar(&staleOnly, "stale-only", false, fmt.Sprintf("Clear only entries older than %d days", ratingCacheTTLDays))
	return cmd
}
