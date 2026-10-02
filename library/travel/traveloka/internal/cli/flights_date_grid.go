// Explicit flight date comparisons coordinate real HTTP searches and persist public snapshots.
// pp:data-source live
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelFlightsDateGridCmd(f *rootFlags) *cobra.Command {
	q := traveloka.Query{Kind: "flights"}
	var departures, returns, dbPath string
	var maxCells int
	cmd := &cobra.Command{Use: "date-grid", Short: "Compare bounded explicit flight dates using authoritative source trip totals",
		Long: "Use this command to compare live flight results across explicit date combinations. Do NOT use this command to rank offers from one existing search; use 'traveloka-pp-cli flights shortlist' instead.",
		Example: strings.Trim(`
  traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent
  traveloka-pp-cli flights date-grid --origin SIN --destination BKK --depart-dates 2026-11-20 --return-dates 2026-11-27,2026-11-28 --adults 2 --max-cells 2 --limit 1 --agent
`, "\n"),
		Annotations: novelAnnotations("live", "--origin=SIN;--destination=CGK;--depart-dates=2026-11-20;--limit=1;--max-candidates=1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if travelokaBareHelp(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "flights date-grid")
			}
			if e := novelNoArgs(args); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := travelokaMode(f, "live"); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := fillShopper(cmd, &q); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := travelokaBounds(q.Limit, 20); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if q.MaxCandidates < 1 || q.MaxCandidates > 10 {
				return travelokaFail(cmd, f, novelInvalid("--max-candidates must be between 1 and 10"))
			}
			queries, e := travelokacompare.FlightGrid(q, departures, returns, maxCells, time.Now())
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			// Validate all requested cells before lowering real network work under harness deadlines.
			if cliutil.IsDogfoodEnv() {
				for i := range queries {
					queries[i].MaxCandidates = 1
					queries[i].Limit = 1
				}
			}
			ctx, cancel := novelGridContext(cmd, f)
			defer cancel()
			src, e := newTravelokaClient(cmd, f)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			// pp:client-call
			result := travelokacompare.RunGrid(ctx, queries, novelGridAttempts(len(queries)), src.SearchFlights, func(ctx context.Context, s *traveloka.Snapshot) error {
				return travelokaSave(ctx, cmd, f, s, dbPath, "")
			})
			return novelGridOutput(cmd, f, result)
		}}
	cmd.Flags().StringVar(&q.Origin, "origin", "", "Three-letter source origin airport code returned by resolve")
	cmd.Flags().StringVar(&q.Destination, "destination", "", "Three-letter source destination airport code returned by resolve")
	cmd.Flags().StringVar(&departures, "depart-dates", "", "Explicit comma-separated ISO departure dates; all cells are validated before retrieval")
	cmd.Flags().StringVar(&returns, "return-dates", "", "Optional comma-separated ISO return dates forming explicit Cartesian date pairs")
	cmd.Flags().StringVar(&q.Cabin, "cabin", "ECONOMY", "Source cabin ECONOMY, PREMIUM_ECONOMY, BUSINESS or FIRST")
	cmd.Flags().IntVar(&q.Adults, "adults", 1, "Number of adults held fixed across all date cells")
	cmd.Flags().IntVar(&q.Children, "children", 0, "Number of children held fixed across all date cells")
	cmd.Flags().IntVar(&q.Infants, "infants", 0, "Number of infants held fixed across all date cells")
	cmd.Flags().IntVar(&q.Limit, "limit", 2, "Maximum authoritative priced offers returned per date cell (1 to 20)")
	cmd.Flags().IntVar(&q.MaxCandidates, "max-candidates", 2, "Maximum source outbound candidates priced per cell (1 to 10)")
	cmd.Flags().IntVar(&maxCells, "max-cells", 4, "Maximum explicit date cells allowed before retrieval (1 to 9)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite history path for normalized public per-cell snapshots")
	return cmd
}
