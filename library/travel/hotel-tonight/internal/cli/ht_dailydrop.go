// `daily-drop` — capture HotelTonight's signature once-a-day flash deal for a
// market and, with --history, read the longitudinal record the app erases.
// Hand-authored (survives `generate --force`).
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newDailyDropCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng    string
		metro       int
		rooms       int
		showHistory bool
	)

	cmd := &cobra.Command{
		Use:   "daily-drop",
		Short: "Capture today's Daily Drop flash deals, or read the recorded Daily Drop history",
		Long: "HotelTonight's Daily Drop is a once-a-day flash deal the app makes ephemeral. This command pulls " +
			"the current deals for a market and surfaces any tagged as a Daily Drop, recording them. With " +
			"--history it instead reads every Daily Drop you've recorded over time from the local store. Some " +
			"Daily Drops are app-unlock-only, so the live feed may not always expose one.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli daily-drop --metro 1
  hotel-tonight-pp-cli daily-drop --metro 1 --history --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if showHistory {
				return runDailyDropHistory(cmd, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rlat, rlng, _, err := resolveGeo(c, metro, lat, lng)
			if err != nil {
				return err
			}
			// Fetch inventory to record normal deals and learn the market's
			// seo_slug + current day, then read the Daily Drop from the
			// server-rendered results page (the app hides it behind a
			// "slide to unlock" gate, but the page embeds the full deal).
			inv, err := fetchAndRecord(cmd.Context(), cmd, c, rlat, rlng, "", "", rooms)
			if err != nil {
				return err
			}
			// The results page keys the Daily Drop on the calendar check-in
			// date ("tonight"), which is today's local date — not the API's
			// `current_day`, whose business-day clock can lag a day behind.
			startDate := timeNow().Format("2006-01-02")
			fd, ok, err := fetchFeaturedDeal(cmd.Context(), inv.PrimaryMarket.SeoSlug, startDate)
			if err != nil {
				return err
			}

			result := map[string]any{
				"market":    inv.PrimaryMarket.CityName,
				"check_in":  inv.CurrentDay,
				"available": ok,
			}
			var rows [][]string
			if ok {
				dd := fd.view()
				result["daily_drop"] = dd
				if s, serr := openPriceStore(cmd.Context()); serr == nil {
					_ = recordDailyDrop(cmd.Context(), s.DB(), numI(inv.PrimaryMarket.ID), inv.PrimaryMarket.CityName, inv.CurrentDay, dd)
					s.Close()
				}
				rows = [][]string{{dd.Hotel, money(dd.Price), money(dd.Was), itoaPct(dd.PctOff), dd.State}}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"No available Daily Drop for %s right now (today's may already be unlocked or sold out).\n", inv.PrimaryMarket.CityName)
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Hotel", "Price", "Was", "Off", "State"}, rows)
		},
	}

	cmd.Flags().StringVar(&lat, "lat", "", "Latitude of the search center")
	cmd.Flags().StringVar(&lng, "lng", "", "Longitude of the search center")
	cmd.Flags().IntVar(&metro, "metro", 0, "Market id (see 'markets list'); alternative to --lat/--lng")
	cmd.Flags().IntVar(&rooms, "rooms", 0, "Number of rooms")
	cmd.Flags().BoolVar(&showHistory, "history", false, "Read recorded Daily Drop history from the local store instead of fetching live")
	return cmd
}

// runDailyDropHistory reads recorded daily_drop snapshots from the store.
func runDailyDropHistory(cmd *cobra.Command, flags *rootFlags) error {
	s, err := openPriceStore(cmd.Context())
	if err != nil {
		return err
	}
	defer s.Close()

	rows, err := queryDailyDropHistory(cmd.Context(), s.DB())
	if err != nil {
		return err
	}
	result := map[string]any{"count": len(rows), "history": rows}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(),
			"No Daily Drop history recorded yet. Run 'daily-drop --metro <id>' over time to build one.")
	}
	tableRows := make([][]string, 0, len(rows))
	for _, r := range rows {
		tableRows = append(tableRows, []string{r.ObservedAt, r.HotelName, r.Market, money(r.Price)})
	}
	return emitJSONOrTable(cmd, flags, result,
		[]string{"Observed", "Hotel", "Market", "Price"}, tableRows)
}

// htDailyDropRow is one recorded Daily Drop observation.
type htDailyDropRow struct {
	ObservedAt string  `json:"observed_at"`
	HotelName  string  `json:"hotel_name"`
	Market     string  `json:"market"`
	Price      float64 `json:"price"`
	PctOff     int     `json:"pct_off"`
}

func queryDailyDropHistory(ctx context.Context, db *sql.DB) ([]htDailyDropRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT observed_at, hotel_name, market_name, customer_price, pct_off
		FROM ht_snapshots
		WHERE deal_type = ?
		ORDER BY observed_at DESC`, dailyDropDealType)
	if err != nil {
		return nil, fmt.Errorf("query daily drop history: %w", err)
	}
	defer rows.Close()

	var out []htDailyDropRow
	for rows.Next() {
		var (
			observedAt, hotelName, market sql.NullString
			price                         sql.NullFloat64
			pctOffV                       sql.NullInt64
		)
		if err := rows.Scan(&observedAt, &hotelName, &market, &price, &pctOffV); err != nil {
			return nil, fmt.Errorf("scan daily drop row: %w", err)
		}
		out = append(out, htDailyDropRow{
			ObservedAt: observedAt.String,
			HotelName:  hotelName.String,
			Market:     market.String,
			Price:      price.Float64,
			PctOff:     int(pctOffV.Int64),
		})
	}
	return out, rows.Err()
}
