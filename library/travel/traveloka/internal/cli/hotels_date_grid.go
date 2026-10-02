// Explicit equal-length hotel stay comparisons use real room/rate-plan retrievals.
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

func newNovelHotelsDateGridCmd(f *rootFlags) *cobra.Command {
	q := traveloka.Query{Kind: "rooms"}
	var stays, ages, dbPath string
	var maxCells int
	cmd := &cobra.Command{Use: "date-grid", Short: "Compare one property's source room rates across explicit equal-length stays",
		Long: "Use this command to compare live offers for a property across explicit stay dates. Do NOT use this command to compare cancellation price differences within one stay; use 'traveloka-pp-cli hotels flexibility' instead.",
		Example: strings.Trim(`
  traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent
  traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08 --adults 3 --rooms 2 --children 1 --child-ages 8 --agent
`, "\n"),
		Annotations: novelAnnotations("live", "--property-id=9000000001714;--stays=2027-01-06:2027-01-08;--adults=2;--rooms=1;--limit=2"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if travelokaBareHelp(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "hotels date-grid")
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
			var e error
			q.ChildAges, e = parseTravelokaAges(ages, q.Children)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := travelokaBounds(q.Limit, 50); e != nil {
				return travelokaFail(cmd, f, e)
			}
			queries, e := travelokacompare.HotelGrid(q, stays, maxCells, time.Now())
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if cliutil.IsDogfoodEnv() {
				for i := range queries {
					if queries[i].Limit > 2 {
						queries[i].Limit = 2
					}
				}
			}
			ctx, cancel := novelGridContext(cmd, f)
			defer cancel()
			src, e := newTravelokaClient(cmd, f)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			// pp:client-call
			result := travelokacompare.RunGrid(ctx, queries, novelGridAttempts(len(queries)), src.HotelRooms, func(ctx context.Context, s *traveloka.Snapshot) error {
				return travelokaSave(ctx, cmd, f, s, dbPath, "")
			})
			result.Note += " Room/rate IDs are retained; differing identities across dates are disclosed and not treated as identical products."
			return novelGridOutput(cmd, f, result)
		}}
	cmd.Flags().StringVar(&q.PropertyID, "property-id", "", "Source property ID returned by resolve or a dated hotel search")
	cmd.Flags().StringVar(&q.PropertyName, "name", "", "Optional source property display name; resolved by Traveloka when omitted")
	cmd.Flags().StringVar(&stays, "stays", "", "Comma-separated explicit check-in:check-out ISO date pairs with equal night lengths")
	cmd.Flags().IntVar(&q.Adults, "adults", 2, "Total adults held fixed across all stay cells")
	cmd.Flags().IntVar(&q.Children, "children", 0, "Total children held fixed across all stay cells")
	cmd.Flags().StringVar(&ages, "child-ages", "", "One comma-separated age per child, held fixed across stays")
	cmd.Flags().IntVar(&q.Rooms, "rooms", 1, "Number of rooms held fixed across all stay cells")
	cmd.Flags().IntVar(&q.Limit, "limit", 10, "Maximum source room/rate plans returned per stay cell (1 to 50)")
	cmd.Flags().IntVar(&maxCells, "max-cells", 4, "Maximum explicit stay cells allowed before retrieval (1 to 9)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite history path for normalized public per-cell room snapshots")
	return cmd
}
