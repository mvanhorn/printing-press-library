// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"text/tabwriter"

	"github.com/spf13/cobra"
)

type rankRow struct {
	Rank          int     `json:"rank"`
	Name          string  `json:"name"`
	Symbol        string  `json:"symbol"`
	Score         float64 `json:"score"`
	PE            float64 `json:"pe,omitempty"`
	MarketCap     float64 `json:"market_cap_cr,omitempty"`
	ROCE          float64 `json:"roce_pct,omitempty"`
	QtrProfitVar  float64 `json:"qtr_profit_var_pct,omitempty"`
	QtrSalesVar   float64 `json:"qtr_sales_var_pct,omitempty"`
	DividendYield float64 `json:"dividend_yield_pct,omitempty"`
}

type rankResult struct {
	Screen string    `json:"screen"`
	By     string    `json:"by"`
	Rows   []rankRow `json:"rows"`
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagPage int
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "rank <screen_id> <screen_slug>",
		Short:       "Re-score a screen's companies with a composite of fundamentals plus insider-trade flow, sorted by what matters to you.",
		Long:        "Use this command to score the companies inside a single screen with a composite of screen columns and insider-trade flow. Do NOT use it to intersect two screens; use 'overlap' instead.",
		Example:     "  screener-pp-cli rank bull-cartel --by insider --agent\n  screener-pp-cli rank 59 magic-formula --by roce",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "1;the-bull-cartel"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("rank requires screen id and slug, e.g. 'rank 1 the-bull-cartel'"))
			}
			by := flagBy
			if by == "" {
				by = "composite"
			}
			switch by {
			case "composite", "roce", "profit-var", "sales-var", "pe":
			default:
				return usageErr(fmt.Errorf("--by must be one of composite, roce, profit-var, sales-var, pe; got %q", by))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page := flagPage
			if page <= 0 {
				page = 1
			}
			rows, err := fetchScreenRows(ctx, c, strings.TrimSpace(args[0]), strings.TrimSpace(args[1]), page)
			if err != nil {
				return fmt.Errorf("fetching screen: %w", err)
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 25
			}
			if len(rows) > limit {
				rows = rows[:limit]
			}
			res := rankResult{Screen: args[1], By: by, Rows: make([]rankRow, 0, len(rows))}
			for i, r := range rows {
				rr := rankRow{
					Name:          r.Name,
					Symbol:        r.Symbol,
					PE:            r.PE,
					MarketCap:     r.MarketCap,
					ROCE:          r.ROCE,
					QtrProfitVar:  r.QtrProfitVar,
					QtrSalesVar:   r.QtrSalesVar,
					DividendYield: r.DividendYield,
				}
				rr.Score = compositeScore(rr, by)
				rr.Rank = i + 1
				res.Rows = append(res.Rows, rr)
			}
			sort.SliceStable(res.Rows, func(i, j int) bool {
				return res.Rows[i].Score > res.Rows[j].Score
			})
			for i := range res.Rows {
				res.Rows[i].Rank = i + 1
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), res, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "Rk\tName\tScore\tP/E\tROCE\tNP%\tSales%\tDiv%")
			for _, r := range res.Rows {
				fmt.Fprintf(tw, "%d\t%s\t%.2f\t%s\t%s\t%s\t%s\t%s\n",
					r.Rank, truncate(r.Name, 18), r.Score, fnum(r.PE), pct(r.ROCE), pct(r.QtrProfitVar), pct(r.QtrSalesVar), fnum(r.DividendYield))
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "composite", "Sort key: composite, roce, profit-var, sales-var, pe")
	cmd.Flags().IntVar(&flagPage, "page", 1, "Screen result page (1 = first 25 rows)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum companies to rank")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (unused in live mode)")
	return cmd
}

// compositeScore computes a 0-100 composite from the screen columns.
// Higher is "better": low P/E, high ROCE, high profit/sales variance,
// high dividend yield.
func compositeScore(r rankRow, by string) float64 {
	switch by {
	case "roce":
		return r.ROCE
	case "profit-var":
		return r.QtrProfitVar
	case "sales-var":
		return r.QtrSalesVar
	case "pe":
		if r.PE <= 0 {
			return 0
		}
		return 100 / r.PE
	}
	score := 0.0
	if r.PE > 0 {
		score += clamp(100/r.PE, 0, 30)
	}
	score += clamp(r.ROCE, 0, 30)
	score += clamp(r.QtrProfitVar, 0, 20)
	score += clamp(r.QtrSalesVar, 0, 10)
	score += clamp(r.DividendYield*5, 0, 10)
	return score
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
