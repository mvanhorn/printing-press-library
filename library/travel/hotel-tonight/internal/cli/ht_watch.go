// `watch` — snapshot a location's deals now and report rooms below a price
// threshold or that dropped since the last recorded run. The diff against
// prior snapshots is the thing the app cannot do (it keeps no price memory).
// Hand-authored (survives `generate --force`).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// htWatchHit is one deal flagged by watch, with the prior price (if any) and
// the reason it surfaced.
type htWatchHit struct {
	htDeal
	PreviousPrice float64  `json:"previous_price,omitempty"`
	DroppedBy     float64  `json:"dropped_by,omitempty"`
	Reasons       []string `json:"reasons"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng string
		metro    int
		when     string
		rooms    int
		category string
		below    float64
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Snapshot a location's deals and flag rooms below a threshold or dropped since last run",
		Long: "Record the current deals for a location and report which rooms are now below your price " +
			"threshold (--below) or have dropped in price since the last recorded run. HotelTonight keeps no " +
			"price history, so the drop comparison is only possible because this command persists every run " +
			"to a local store. Run it on a cron to catch last-minute drops while you're away.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli watch --lat 37.7749 --lng -122.4194 --below 150
  hotel-tonight-pp-cli watch --metro 1 --when tonight --category solid,hip --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cats, err := parseCategories(category)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rlat, rlng, _, err := resolveGeo(c, metro, lat, lng)
			if err != nil {
				return err
			}
			ci, co := computeDates(when, timeNow())

			// Capture prior prices BEFORE recording this run so the just-written
			// snapshot can't shadow the previous observation.
			runStart := timeNow()
			var prior map[string]float64
			if s, err := openPriceStore(cmd.Context()); err == nil {
				var perr error
				prior, perr = lastPriceByHotel(cmd.Context(), s.DB(), runStart)
				s.Close()
				if perr != nil {
					// Non-fatal: warn and continue with no baseline rather than
					// silently treating a query failure as "nothing dropped".
					fmt.Fprintf(cmd.ErrOrStderr(), "note: prior-price lookup failed (%v); reporting threshold matches only\n", perr)
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: price store unavailable (%v); reporting threshold matches only\n", err)
			}
			if prior == nil {
				prior = map[string]float64{}
			}

			inv, err := fetchAndRecord(cmd.Context(), cmd, c, rlat, rlng, ci, co, rooms)
			if err != nil {
				return err
			}

			var hits []htWatchHit
			for _, d := range dealsFrom(inv, cats, 0, "price") {
				var reasons []string
				if below > 0 && d.Price > 0 && d.Price <= below {
					reasons = append(reasons, "below "+money(below))
				}
				var prev, droppedBy float64
				if p, ok := prior[d.HotelName]; ok && p > 0 && d.Price > 0 && d.Price < p {
					prev = p
					droppedBy = p - d.Price
					reasons = append(reasons, "dropped "+money(droppedBy)+" since last run")
				}
				if len(reasons) == 0 {
					continue
				}
				hits = append(hits, htWatchHit{htDeal: d, PreviousPrice: prev, DroppedBy: droppedBy, Reasons: reasons})
			}

			result := map[string]any{
				"market":    inv.PrimaryMarket.CityName,
				"check_in":  inv.CurrentDay,
				"below":     below,
				"hits":      hits,
				"hit_count": len(hits),
				"baseline":  len(prior) > 0,
			}
			rows := make([][]string, 0, len(hits))
			for _, h := range hits {
				rows = append(rows, []string{h.HotelName, h.Neighborhood, money(h.Price), strings.Join(h.Reasons, "; ")})
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Hotel", "Neighborhood", "Price", "Why"}, rows)
		},
	}

	cmd.Flags().StringVar(&lat, "lat", "", "Latitude of the watched location")
	cmd.Flags().StringVar(&lng, "lng", "", "Longitude of the watched location")
	cmd.Flags().IntVar(&metro, "metro", 0, "Market id to watch (see 'markets list'); alternative to --lat/--lng")
	cmd.Flags().StringVar(&when, "when", "tonight", "When to stay: tonight, tomorrow, or weekend")
	cmd.Flags().IntVar(&rooms, "rooms", 0, "Number of rooms")
	cmd.Flags().StringVar(&category, "category", "", categoryFlagHelp)
	cmd.Flags().Float64Var(&below, "below", 0, "Flag rooms at or below this nightly price")
	return cmd
}
