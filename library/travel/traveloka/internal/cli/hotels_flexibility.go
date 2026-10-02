// Same-room cancellation comparisons preserve exact source terms and unknowns.
// pp:data-source local
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelHotelsFlexibilityCmd(f *rootFlags) *cobra.Command {
	var file, id, dbPath, maxAgeValue string
	var limit, scan int
	cmd := &cobra.Command{Use: "flexibility", Short: "Compare source stay-price differences for exactly matched cancellation alternatives",
		Long: "Use this command to compare cancellation price differences among retrieved, comparable rate plans for the same room and stay. Do NOT use this command to compare alternative stay dates; use 'traveloka-pp-cli hotels date-grid' instead.",
		Example: strings.Trim(`
  traveloka-pp-cli hotels flexibility --snapshot /private/tmp/traveloka-rooms.json --limit 10 --agent
  traveloka-pp-cli hotels flexibility --db /private/tmp/traveloka-public.sqlite --max-age 24h --agent
`, "\n"),
		Annotations: novelAnnotations("local", "--data-source=local"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if travelokaBareHelp(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "hotels flexibility")
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
			s, e := novelLoad(ctx, cmd, dbPath, file, id, "rooms", maxAge)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if s == nil {
				return novelEmpty(cmd, f, []string{"pairs", "unpaired"}, "Run hotels rooms to save real source rate plans")
			}
			result, e := travelokacompare.Flexibility(s, limit, scan)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			return f.printJSON(cmd, map[string]any{"status": "saved_snapshot_flexibility", "query": s.Query, "snapshot_id": s.ID, "retrieved_at": s.RetrievedAt, "freshness": "saved_snapshot", "indicative": true, "price_basis": "party_stay_total", "pairs": result.Pairs, "unpaired": result.Unpaired, "pair_count": result.PairCount, "scanned_offers": result.Scanned, "truncated": result.Truncated, "source_coverage": s.Coverage, "note": result.Note})
		}}
	cmd.Flags().StringVar(&file, "snapshot", "", "Optional public source snapshot JSON file; otherwise read SQLite history")
	cmd.Flags().StringVar(&id, "snapshot-id", "", "Optional exact SQLite snapshot ID; omitted selects the latest matching kind")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read-only public SQLite snapshot history path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum returned entries per named result section (1 to 100)")
	cmd.Flags().IntVar(&scan, "max-scan-records", 500, "Maximum retrieved offers examined from the selected snapshot (1 to 1000)")
	cmd.Flags().StringVar(&maxAgeValue, "max-age", "24h", "Warn on old local snapshots; accepts 7d/1w; zero disables hints")
	return cmd
}
