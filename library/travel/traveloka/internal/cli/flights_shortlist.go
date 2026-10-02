// Flight Pareto frontier reads public snapshots without fetching fresh inventory.
// pp:data-source local
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelFlightsShortlistCmd(f *rootFlags) *cobra.Command {
	var file, id, dbPath, maxAgeValue string
	var limit, scan int
	cmd := &cobra.Command{Use: "shortlist", Short: "Find nondominated source flight price, stops and duration trade-offs",
		Long: "Use this command to identify price, stops and duration trade-offs within an existing matched-context flight search. Do NOT use this command to search alternative dates; use 'traveloka-pp-cli flights date-grid' instead.",
		Example: strings.Trim(`
  traveloka-pp-cli flights shortlist --snapshot /private/tmp/traveloka-flight.json --limit 10 --agent
  traveloka-pp-cli flights shortlist --db /private/tmp/traveloka-public.sqlite --max-scan-records 500 --agent
`, "\n"),
		Annotations: novelAnnotations("local", "--data-source=local"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if travelokaBareHelp(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "flights shortlist")
			}
			if e := novelNoArgs(args); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := travelokaMode(f, "local"); e != nil {
				return travelokaFail(cmd, f, e)
			}
			maxAge, e := novelParseMaxAge(maxAgeValue)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := novelLocalOptions(limit, scan, maxAge); e != nil {
				return travelokaFail(cmd, f, e)
			}
			ctx, cancel := boundCtx(cmd.Context(), f)
			defer cancel()
			s, e := novelLoad(ctx, cmd, dbPath, file, id, "flights", maxAge)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if s == nil {
				return novelEmpty(cmd, f, []string{"offers", "unknown_dimensions"}, "Run flights search to save a real source flight snapshot")
			}
			result, e := travelokacompare.Frontier(s, limit, scan)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			return f.printJSON(cmd, map[string]any{"status": "saved_snapshot_frontier", "query": s.Query, "snapshot_id": s.ID, "retrieved_at": s.RetrievedAt, "freshness": "saved_snapshot", "indicative": true, "price_basis": "party_trip_total", "offers": result.Offers, "unknown_dimensions": result.Unknown, "scanned_offers": result.Scanned, "frontier_count": result.FrontierCount, "dominated_offers": result.Dominated, "truncated": result.Truncated, "source_coverage": s.Coverage, "note": result.Note})
		}}
	cmd.Flags().StringVar(&file, "snapshot", "", "Optional public source snapshot JSON file; otherwise read SQLite history")
	cmd.Flags().StringVar(&id, "snapshot-id", "", "Optional exact SQLite snapshot ID; omitted selects the latest matching kind")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read-only public SQLite snapshot history path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned entries per named result section (1 to 100)")
	cmd.Flags().IntVar(&scan, "max-scan-records", 500, "Maximum retrieved offers examined from the selected snapshot (1 to 1000)")
	cmd.Flags().StringVar(&maxAgeValue, "max-age", "24h", "Warn on old local snapshots; accepts 7d/1w; zero disables hints")
	return cmd
}
