// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	var market string
	var limit int
	cmd := &cobra.Command{
		Use:   "history <article-id-or-words>",
		Short: "Prices an article had over time in your markets, from your own searches and offer lookups.",
		Long: "Reads the local price store that 'products search', 'offers', 'deals check' and 'basket add' fill.\n" +
			"Give an article id for one article, or words to match titles. Shows each day's price per market and source,\n" +
			"plus the lowest and highest price seen. No network access.",
		Example:     "  edeka-pp-cli price history 5915407005MXST --json\n  edeka-pp-cli price history milch",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "article=milch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "price history")
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			q := joinArgs(args)
			if localDataDisabled() && edekaDBMissing() {
				out := map[string]any{"query": q, "points": []any{}, "count": 0, "hint": "local data is off (" + noLocalDataEnvVar + "=1) and no price store exists, so no prices are recorded"}
				return emitThen(cmd, flags, out, notFoundErr(fmt.Errorf("no recorded prices for %q", q)))
			}
			db, err := openEdekaDB(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			where := "article_id = ?"
			arg := q
			if !edeka.LooksLikeArticleID(q) {
				where = "lower(title) LIKE ?"
				arg = "%" + strings.ToLower(q) + "%"
			}
			sqlq := `SELECT article_id, MAX(title), MIN(price_cents), MAX(unit_price), MAX(on_offer), GROUP_CONCAT(DISTINCT source), market, seen_on FROM edeka_prices WHERE ` + where
			params := []any{arg}
			if market != "" {
				sqlq += " AND market = ?"
				host, err := edeka.NormalizeMarket(market)
				if err != nil {
					return usageErr(err)
				}
				params = append(params, edeka.MarketSlug(host))
			}
			sqlq += " GROUP BY article_id, market, seen_on ORDER BY article_id, seen_on DESC, market LIMIT ?"
			params = append(params, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), sqlq, params...)
			if err != nil {
				return configErr(err)
			}
			type point struct {
				ArticleID  string `json:"article_id"`
				Title      string `json:"title"`
				PriceCents int    `json:"price_cents"`
				Price      string `json:"price"`
				UnitPrice  string `json:"unit_price,omitempty"`
				OnOffer    bool   `json:"on_offer"`
				Source     string `json:"sources"`
				Market     string `json:"market"`
				Day        string `json:"day"`
			}
			var pts []point
			for rows.Next() {
				var p point
				var offer int
				if err := rows.Scan(&p.ArticleID, &p.Title, &p.PriceCents, &p.UnitPrice, &offer, &p.Source, &p.Market, &p.Day); err != nil {
					_ = rows.Close()
					return configErr(err)
				}
				p.OnOffer = offer == 1
				p.Price = edeka.FormatCents(p.PriceCents)
				pts = append(pts, p)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return configErr(err)
			}
			_ = rows.Close()
			lo, hi := 0, 0
			for i, p := range pts {
				if i == 0 || p.PriceCents < lo {
					lo = p.PriceCents
				}
				if p.PriceCents > hi {
					hi = p.PriceCents
				}
			}
			out := map[string]any{"query": q, "points": pts, "count": len(pts)}
			if len(pts) > 0 {
				out["lowest"] = edeka.FormatCents(lo)
				out["highest"] = edeka.FormatCents(hi)
			} else {
				out["points"] = []point{}
				out["hint"] = "no prices recorded yet: run 'products search', 'offers' or 'deals check' first"
				return emitThen(cmd, flags, out, notFoundErr(fmt.Errorf("no recorded prices for %q", q)))
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				if len(pts) == 0 {
					fmt.Fprintln(w, "No prices recorded yet for this query. Run 'products search' or 'offers' first.")
					return
				}
				for _, p := range pts {
					off := ""
					if p.OnOffer {
						off = " [offer]"
					}
					fmt.Fprintf(w, "%s  %-16s %-44s %9s  %s (%s)%s\n", p.Day, p.ArticleID, truncate(p.Title, 44), p.Price, p.Market, p.Source, off)
				}
				fmt.Fprintf(w, "Lowest %s, highest %s.\n", edeka.FormatCents(lo), edeka.FormatCents(hi))
			})
		},
	}
	cmd.Flags().StringVar(&market, "market", "", "Only this market (subdomain)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum rows")
	return cmd
}
