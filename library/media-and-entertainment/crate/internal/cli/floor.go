// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// floor reads the shelf from the local store and prices it from the
// marketplace, serving cached prices when they are fresh enough.
// pp:data-source auto
func newNovelFloorCmd(flags *rootFlags) *cobra.Command {
	var (
		user     string
		limit    int
		currency string
		maxAge   time.Duration
		top      int
	)
	cmd := &cobra.Command{
		Use:   "floor",
		Short: "Totals the lowest current asking price across your collection, as a floor rather than an appraisal",
		Long: strings.Trim(`
A floor on what the collection is worth. Not a valuation.

Each record contributes the lowest price a seller is currently ASKING on the
Discogs marketplace. That is not a sale price, not a median, and not what a
copy in your condition would fetch. It is the cheapest anyone is willing to
part with one today, summed. Treat it as a lower bound and nothing more.

Discogs' own valuation endpoint needs a token; this reconstructs a floor
keylessly from per-release marketplace stats. That costs one request per
record against a 25-per-minute keyless limit, so pricing is bounded by
--limit and cached. Every run reports how many records it could not price —
a total over half a collection is not a total.
`, "\n"),
		Example: strings.Trim(`
  crate-pp-cli floor --user example
  crate-pp-cli floor --user example --limit 50 --currency GBP
  crate-pp-cli floor --user example --top 10 --json
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

			recs, err := loadShelf(ctx, cmd, c, h, u, false)
			if err != nil {
				return err
			}

			prices, fetched, unpriced, err := priceRecords(ctx, cmd, c, h, recs, limit, currency, maxAge)
			if err != nil {
				return err
			}

			type line struct {
				Title    string  `json:"title"`
				Artist   string  `json:"artist"`
				Lowest   float64 `json:"lowest"`
				Currency string  `json:"currency"`
				ForSale  int     `json:"num_for_sale"`
			}
			// Totals are kept per returned currency. Discogs reports whatever
			// currency it chose, and adding GBP to JPY produces a number that
			// is wrong in every currency while being labelled with one of them.
			totals := map[string]float64{}
			var priced, noneForSale int
			var lines []line
			for _, r := range recs {
				p, ok := prices[r.ReleaseID]
				if !ok {
					continue
				}
				if !p.HasPrice {
					noneForSale++
					continue
				}
				totals[p.Currency] += p.Lowest
				priced++
				lines = append(lines, line{
					Title: r.Title, Artist: r.ArtistLine(),
					Lowest: p.Lowest, Currency: p.Currency, ForSale: p.NumForSale,
				})
			}
			sort.SliceStable(lines, func(i, j int) bool { return lines[i].Lowest > lines[j].Lowest })

			res := map[string]any{
				"user": u, "records": len(recs),
				"priced": priced, "unpriced": unpriced,
				"none_for_sale": noneForSale, "fetched_this_run": fetched,
				"floor_totals_by_currency": totals,
				"is_appraisal":             false,
				"basis":                    "sum of the lowest current asking price per record; a lower bound, not a valuation",
			}
			if top > 0 && len(lines) > top {
				lines = lines[:top]
			}
			res["most_expensive"] = lines

			if flags.asJSON {
				return flags.printJSON(cmd, res)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, bold(fmt.Sprintf("%s — collection price floor", u)))
			curs := make([]string, 0, len(totals))
			for c := range totals {
				curs = append(curs, c)
			}
			sort.Strings(curs)
			for _, c := range curs {
				label := "floor total"
				if len(curs) > 1 {
					label = "floor total (" + c + ")"
				}
				fmt.Fprintf(out, "  %-18s %.2f %s\n", label, totals[c], c)
			}
			if len(curs) > 1 {
				fmt.Fprintf(out, "  %-18s %s\n", "",
					yellow("prices came back in several currencies; they are listed separately rather than added together"))
			}
			fmt.Fprintf(out, "  %-18s %d of %d records\n", "priced", priced, len(recs))
			if noneForSale > 0 {
				fmt.Fprintf(out, "  %-18s %d (nothing currently listed; contributed nothing)\n", "no copies for sale", noneForSale)
			}
			if unpriced > 0 {
				fmt.Fprintf(out, "  %-18s %s\n", "not priced",
					yellow(fmt.Sprintf("%d records — this total covers only part of the collection; re-run to extend", unpriced)))
			}
			fmt.Fprintf(out, "  %-18s %d\n", "fetched this run", fetched)
			fmt.Fprintf(out, "\n  This is the sum of the LOWEST price anyone is currently asking.\n")
			fmt.Fprintf(out, "  It is a lower bound, not an appraisal, and not what your copies would sell for.\n")

			if len(lines) > 0 {
				fmt.Fprintln(out, "")
				rows := make([][]string, 0, len(lines))
				for _, l := range lines {
					rows = append(rows, []string{
						l.Artist, l.Title,
						fmt.Sprintf("%.2f %s", l.Lowest, l.Currency),
						fmt.Sprintf("%d", l.ForSale),
					})
				}
				if err := flags.printTable(cmd, []string{"ARTIST", "TITLE", "LOWEST ASK", "FOR SALE"}, rows); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", userFlagHelp)
	cmd.Flags().IntVar(&limit, "limit", 10, "How many uncached records to price this run; each costs one request against Discogs' 25-per-minute keyless limit")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for prices, e.g. USD, GBP, EUR")
	cmd.Flags().DurationVar(&maxAge, "max-price-age", 24*time.Hour, "Reuse cached prices younger than this")
	cmd.Flags().IntVar(&top, "top", 15, "Show the N most expensive priced records (0 for all)")
	return cmd
}
