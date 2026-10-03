// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelTripCachedCmd(flags *rootFlags) *cobra.Command {
	var keyword, kind, on, amenities string
	var age, limit, maxScan int
	cmd := &cobra.Command{
		Use: "cached [keyword]", Short: "Search selected saved Iko-yo Trip facts by keyword, date, age and amenities with observation times and scan coverage",
		Long: "Read only selected Trip facts previously saved by discover or inspect. This is a partial local collection, not a synchronized catalog. Listing-only facts often lack age and facility evidence; unknown requirements are not confirmed matches. Observation timestamps remain unchanged. Collection provenance is bounded to 5,000 memberships and 100 collections. Constraint inputs are per-invocation and not automatically learned. " + trip.ScopeNote,
		Example: strings.Trim(`
  iko-yo-pp-cli trip cached Mooovi --kind spots --agent
  iko-yo-pp-cli trip cached --kind events --on 2026-11-15 --max-scan-records 500 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "keyword=Mooovi;--kind=spots"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trip cached")
			}
			if len(args) > 1 || len(args) == 1 && keyword != "" {
				return usageErr(fmt.Errorf("provide one keyword as a positional or --query"))
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("trip cached is local-only; use trip discover for live listing reads"))
			}
			if flags.noCache {
				return usageErr(fmt.Errorf("trip cached requires saved facts; omit --no-cache"))
			}
			query := keyword
			if len(args) == 1 {
				query = args[0]
			}
			q := trip.Query{Kind: kind, Keyword: strings.TrimSpace(query), From: on, To: on, AgeMonths: age, Amenities: tripAmenities(amenities), Limit: limit}
			if err := trip.ValidateQuery(q); err != nil {
				return usageErr(err)
			}
			if maxScan < 1 || maxScan > 1000 {
				return usageErr(fmt.Errorf("--max-scan-records must be from 1 to 1000"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openStoreForRead(ctx, "iko-yo-pp-cli")
			if err != nil {
				return tripError(err)
			}
			records := make([]trip.Record, 0)
			coverage := make([]trip.Coverage, 0)
			total := 0
			if db != nil {
				defer db.Close()
				hintIfUnsynced(cmd, db, kind)
				hintIfStale(cmd, db, kind, flags.maxAge)
				records, coverage, err = db.TripRecords(ctx, kind, maxScan)
				if err != nil {
					return tripError(err)
				}
				total, err = db.TripCacheCount(ctx, kind)
				if err != nil {
					return tripError(err)
				}
			}
			for i := range records {
				records[i] = trip.RefreshStatus(records[i], tripToday())
			}
			view := trip.Filter(records, q)
			view.Coverage = coverage
			view.Note += fmt.Sprintf(" Local scan checked %d of %d saved %s records (cap %d); widen --max-scan-records up to 1000. Collection provenance is bounded to 5,000 memberships and 100 collections. Saved records are not source-wide coverage.", view.ScannedRecords, total, kind, maxScan)
			tripSource(cmd, flags, "local")
			return tripPrintDiscovery(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&keyword, "query", "", "Local keyword in saved names, places, and factual excerpts")
	cmd.Flags().StringVar(&kind, "kind", "all", "Saved record kind: spots, events, or all")
	cmd.Flags().StringVar(&on, "on", "", "YYYY-MM-DD date for overlap with saved published event dates")
	cmd.Flags().IntVar(&age, "age-months", -1, "Age in months, zero to 216; omit for no age check")
	cmd.Flags().StringVar(&amenities, "amenities", "", "Required evidence keys: indoor,nursing,changing,stroller, comma separated")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum matching saved records returned, from one to fifty")
	cmd.Flags().IntVar(&maxScan, "max-scan-records", 500, "Maximum saved records examined, independently of output limit")
	return cmd
}
