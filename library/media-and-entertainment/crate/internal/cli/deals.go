// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// deals reads the wantlist from the local store and prices it from the
// marketplace, serving cached prices when they are fresh enough.
// pp:data-source auto
func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var (
		user     string
		under    float64
		limit    int
		currency string
		maxAge   time.Duration
	)
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Cross-references your wantlist against current marketplace listings to show what is cheap and available now",
		Long: strings.Trim(`
What to buy today, out of what you already said you want.

Your wantlist says what you want. Marketplace stats say what things cost.
Neither answers the actual question on its own; this joins them and sorts by
the lowest current asking price.

Prices are the cheapest a seller is currently ASKING, not a sale price. Pricing
costs one request per record against a 25-per-minute keyless limit, so it is
bounded by --limit and cached; the output always says how much of the wantlist
was actually checked.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli deals --user <username>
  crate-pp-cli deals --user <username> --under 20
  crate-pp-cli deals --user <username> --under 25 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			u, err := resolveUser(user)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			h, err := openCrate(ctx)
			if err != nil {
				return err
			}
			defer h.closeFn()

			wants, err := loadShelf(ctx, cmd, c, h, u, true)
			if err != nil {
				return err
			}

			prices, fetched, unpriced, err := priceRecords(ctx, cmd, c, h, wants, limit, currency, maxAge)
			if err != nil {
				return err
			}

			type deal struct {
				ReleaseID int64   `json:"release_id"`
				Title     string  `json:"title"`
				Artist    string  `json:"artist"`
				Year      int     `json:"year"`
				Lowest    float64 `json:"lowest"`
				Currency  string  `json:"currency"`
				ForSale   int     `json:"num_for_sale"`
			}
			var deals []deal
			var checked, unavailable int
			for _, r := range wants {
				p, ok := prices[r.ReleaseID]
				if !ok {
					continue
				}
				checked++
				if !p.HasPrice {
					unavailable++
					continue
				}
				if under > 0 && p.Lowest > under {
					continue
				}
				deals = append(deals, deal{
					ReleaseID: r.ReleaseID, Title: r.Title, Artist: r.ArtistLine(),
					Year: r.Year, Lowest: p.Lowest, Currency: p.Currency, ForSale: p.NumForSale,
				})
			}
			sort.SliceStable(deals, func(i, j int) bool { return deals[i].Lowest < deals[j].Lowest })

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"user": u, "wantlist_size": len(wants), "checked": checked,
					"unpriced": unpriced, "none_for_sale": unavailable,
					"fetched_this_run": fetched, "under": under, "deals": deals,
				})
			}

			out := cmd.OutOrStdout()
			// --limit bounds how many wantlist records get priced this run,
			// so the table is the cheapest of what has been priced so far,
			// not the cheapest on the wantlist. Say so in the headline: the
			// coverage footer alone reads as a caveat rather than a
			// correction to what the table claims.
			if checked < len(wants) {
				fmt.Fprintln(out, bold(fmt.Sprintf("%s — cheapest of the %d wantlist records priced so far", u, checked)))
			} else {
				fmt.Fprintln(out, bold(fmt.Sprintf("%s — wantlist deals", u)))
			}
			if len(deals) == 0 {
				if under > 0 {
					fmt.Fprintf(out, "  nothing on the wantlist is currently for sale under %.2f\n", under)
				} else {
					fmt.Fprintln(out, "  nothing on the wantlist is currently for sale")
				}
			} else {
				rows := make([][]string, 0, len(deals))
				for _, d := range deals {
					y := ""
					if d.Year > 0 {
						y = fmt.Sprintf("%d", d.Year)
					}
					rows = append(rows, []string{
						fmt.Sprintf("%.2f %s", d.Lowest, d.Currency),
						d.Artist, d.Title, y, fmt.Sprintf("%d", d.ForSale),
					})
				}
				if err := flags.printTable(cmd, []string{"LOWEST ASK", "ARTIST", "TITLE", "YEAR", "FOR SALE"}, rows); err != nil {
					return err
				}
			}

			fmt.Fprintf(out, "\n  checked %d of %d wantlist records", checked, len(wants))
			if unpriced > 0 {
				fmt.Fprintf(out, "; %s", yellow(fmt.Sprintf("%d not priced this run — re-run to extend coverage", unpriced)))
			}
			fmt.Fprintln(out, "")
			if unavailable > 0 {
				fmt.Fprintf(out, "  %d checked records have no copies for sale right now\n", unavailable)
			}
			fmt.Fprintln(out, "  Prices are the lowest current asking price, not a sale price.")
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().Float64Var(&under, "under", 0, "Only show records available below this price (0 for no cap)")
	// Ten rather than the full keyless minute-budget: pricing is capped at
	// Discogs' own 25 requests per minute, so a default of 25 makes the very
	// first run sit silent for a full minute. Ten returns in about twenty
	// seconds and the output says how much of the wantlist is still unpriced.
	cmd.Flags().IntVar(&limit, "limit", 10, "How many uncached wantlist records to price this run; each costs one request against Discogs' 25-per-minute keyless limit")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for prices, e.g. USD, GBP, EUR")
	cmd.Flags().DurationVar(&maxAge, "max-price-age", 6*time.Hour, "Reuse cached prices younger than this")
	return cmd
}
