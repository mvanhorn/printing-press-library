// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
)

func newNovelTripDiscoverCmd(flags *rootFlags) *cobra.Command {
	var kind, keyword, from, to string
	var region, prefecture, maxPages, limit int
	cmd := &cobra.Command{
		Use: "discover", Short: "Find Iko-yo Trip events/spots by region, keyword and dates within a bounded listing window",
		Long: "Read Iko-yo Trip's selected family experiences and local events. Keyword and date filters run locally across the reported pages. Listings are publication-ordered and include archived events. A zero result does not mean no matching events exist. Missing or irregular schedules stay unknown. " + trip.ScopeNote,
		Example: strings.Trim(`
  iko-yo-pp-cli trip discover --kind events --region 6 --prefecture 11 --from 2026-11-14 --to 2026-11-15 --max-pages 2 --agent
  iko-yo-pp-cli trip discover --kind spots --query Mooovi --max-pages 1 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--kind=events;--region=6;--prefecture=11;--max-pages=1;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trip discover")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("trip discover accepts flags only; use --query for keywords"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("trip discover requires a live source; use trip cached for saved facts"))
			}
			if cmd.Flags().Changed("max-pages") && cmd.Flags().Changed("max-scan-pages") {
				return usageErr(fmt.Errorf("use only one of --max-pages and --max-scan-pages"))
			}
			q := trip.Query{Kind: kind, Keyword: strings.TrimSpace(keyword), From: from, To: to, AgeMonths: -1, Limit: limit}
			if err := trip.ValidateQuery(q); err != nil {
				return usageErr(err)
			}
			if region < 0 || region > 11 || prefecture < 0 || prefecture > 47 || prefecture > 0 && region == 0 {
				return usageErr(fmt.Errorf("--region must be 1–11; --prefecture must be 1–47 with its source --region"))
			}
			if maxPages < 1 || maxPages > 5 || kind == "all" && maxPages < 2 {
				return usageErr(fmt.Errorf("--max-pages must be 1–5 total; --kind all requires at least 2"))
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
				if kind == "all" {
					maxPages = 2
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ctx, capCancel := context.WithTimeout(ctx, time.Minute)
			defer capCancel()
			v, records, err := trip.New(flags.rateLimit).Discover(ctx, kind, region, prefecture, maxPages, q)
			if err != nil {
				return tripError(err)
			}
			if err = tripSave(ctx, flags, records); err != nil {
				return tripError(err)
			}
			tripSource(cmd, flags, "live")
			return tripPrintDiscovery(cmd, flags, v)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "events", "Listing kind: events, spots, or both with all")
	cmd.Flags().IntVar(&region, "region", 0, "Source region ID 1–11; omit for nationwide listings")
	cmd.Flags().IntVar(&prefecture, "prefecture", 0, "Source prefecture ID 1–47 within the selected region")
	cmd.Flags().StringVar(&keyword, "query", "", "Local keyword in names, places, and published facts")
	cmd.Flags().StringVar(&from, "from", "", "First YYYY-MM-DD date for published event overlap")
	cmd.Flags().StringVar(&to, "to", "", "Last YYYY-MM-DD date for published event overlap")
	cmd.Flags().IntVar(&maxPages, "max-pages", 2, "Total listing pages to scan across kinds, capped at five")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 2, "Alias of --max-pages; bounds scans independently of output")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum matching records returned, from one to fifty")
	return cmd
}
