// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type screenResult struct {
	Expression string  `json:"expression"`
	Sector     string  `json:"sector,omitempty"`
	Sort       string  `json:"sort"`
	Total      int     `json:"total"`
	Matched    int     `json:"matched"`
	Source     string  `json:"source"`
	AsOf       string  `json:"as_of,omitempty"`
	Rows       []ipRow `json:"rows"`
	Note       string  `json:"note,omitempty"`
}

var screenBaseCols = []string{"symbol", "name", "sector", "close", "chg1d", "rs", "rsi14"}

func newNovelScreenCmd(flags *rootFlags) *cobra.Command {
	var flagSector, flagSort, dbPath string
	var flagAsc, flagSaved bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "screen [expression]",
		Short: "Run the site's screener expression language (close > sma50 and rs >= 70)",
		Long: `Run the InvestorPaisa screener expression language over the Nifty 200 metrics table.

Fields are the 48 screener columns (close, sma50, sma200, rs, rsi14, ret1m, pos52, volratio, ...).
Operators: > >= < <= == != + - * /, and, or, not, parentheses. Strings use quotes: sector == 'IT'.

Use this to run an expression over the Nifty 200 metrics, with --sector, --sort and --saved for your own saved expression.
Do NOT use it for a single stock's full picture; use 'quote' instead.
Do NOT use it for ranking today's price or volume moves; use 'movers' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli screen 'close > sma50 and sma50 > sma200 and rs >= 70' --sector IT --agent
  investorpaisa-pp-cli screen 'rsi14 < 35 and pos52 < 20' --sort rsi14 --asc --limit 10
  investorpaisa-pp-cli screen --saved`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "expression=rs >= 70"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "screen")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			expression := strings.TrimSpace(strings.Join(args, " "))
			sector := flagSector
			if flagSaved {
				if flags.dataSource == "local" {
					return usageErr(fmt.Errorf("--saved reads your saved screen from the live API; drop --data-source local or pass the expression directly"))
				}
				saved, savedSector, err := ipSavedScreen(ctx, flags)
				if err != nil {
					return err
				}
				if expression == "" {
					expression = saved
				}
				if sector == "" {
					sector = savedSector
				}
			}
			if expression == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an expression is required (or pass --saved to use your saved screen)"))
			}
			expr, err := parseIPExpr(expression)
			if err != nil {
				return usageErr(fmt.Errorf("invalid expression %q: %w", expression, err))
			}
			loader := newIPLoader(cmd, flags, dbPath)
			defer loader.close()
			rows, src, err := loader.screenerRows(ctx)
			if err != nil {
				return err
			}
			valid := map[string]bool{}
			for _, r := range rows {
				for k := range r {
					valid[k] = true
				}
			}
			if err := expr.validate(valid); err != nil {
				return usageErr(err)
			}
			sortKey := strings.ToLower(flagSort)
			if alias, ok := ipFieldAliases[sortKey]; ok {
				sortKey = alias
			}
			if !valid[sortKey] {
				return usageErr(fmt.Errorf("unknown --sort field %q", flagSort))
			}
			if sortKey == "symbol" || sortKey == "name" || sortKey == "sector" || sortKey == "date" || sortKey == "from" {
				return usageErr(fmt.Errorf("--sort %q is not numeric; sort by a metric such as rs, rsi14, ret1m or volratio", flagSort))
			}
			cols := append([]string(nil), screenBaseCols...)
			for _, f := range append(expr.fields(), sortKey) {
				if f == "symbol" || f == "name" || f == "sector" {
					continue
				}
				dup := false
				for _, c := range cols {
					dup = dup || c == f
				}
				if !dup {
					cols = append(cols, f)
				}
			}
			matched := make([]ipRow, 0)
			for _, r := range rows {
				if sector != "" && !strings.EqualFold(ipStrField(r, "sector"), sector) {
					continue
				}
				if expr.matches(r) {
					matched = append(matched, r)
				}
			}
			sort.SliceStable(matched, func(i, j int) bool {
				a, aok := ipNum(matched[i], sortKey)
				b, bok := ipNum(matched[j], sortKey)
				if aok != bok {
					return aok
				}
				if flagAsc {
					return a < b
				}
				return a > b
			})
			res := screenResult{Expression: expression, Sector: sector, Sort: sortKey, Total: len(rows), Matched: len(matched), Source: src.Source, AsOf: src.AsOf, Rows: make([]ipRow, 0)}
			for i, r := range matched {
				if flagLimit > 0 && i >= flagLimit {
					break
				}
				out := ipRow{}
				for _, c := range cols {
					if v, ok := r[c]; ok {
						if f, isNum := v.(float64); isNum {
							v = ipRound(f, 2)
						}
						out[c] = v
					}
				}
				res.Rows = append(res.Rows, out)
			}
			if len(matched) == 0 {
				res.Note = "no stock matched; loosen the expression or drop --sector"
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%d of %d stocks match  (%s, as of %s)\n", res.Matched, res.Total, res.Source, res.AsOf)
				if len(res.Rows) > 0 {
					items := make([]map[string]any, 0, len(res.Rows))
					for _, r := range res.Rows {
						items = append(items, r)
					}
					return printAutoTable(cmd.OutOrStdout(), items)
				}
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&flagSector, "sector", "", "Only consider stocks in this sector (e.g. IT, FMCG)")
	cmd.Flags().StringVar(&flagSort, "sort", "rs", "Field to sort by")
	cmd.Flags().BoolVar(&flagAsc, "asc", false, "Sort ascending instead of descending")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return (0 for all)")
	cmd.Flags().BoolVar(&flagSaved, "saved", false, "Use the screener expression saved on your InvestorPaisa account")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}

// ipSavedScreen reads the expression and sector saved in /api/me/layout.
func ipSavedScreen(ctx context.Context, flags *rootFlags) (string, string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", "", err
	}
	data, err := c.GetNoCache(ctx, "/api/me/layout", nil)
	if err != nil {
		return "", "", classifyAPIErrorOnly(err)
	}
	var layout struct {
		Screener struct {
			Expression string  `json:"expression"`
			Sector     *string `json:"sector"`
		} `json:"screener"`
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		return "", "", fmt.Errorf("unexpected layout response: %w", err)
	}
	sector := ""
	if layout.Screener.Sector != nil {
		sector = *layout.Screener.Sector
	}
	return layout.Screener.Expression, sector, nil
}
