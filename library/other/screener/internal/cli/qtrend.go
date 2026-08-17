// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/other/screener/internal/client"

	"github.com/spf13/cobra"
)

type qtrendRow struct {
	Period        string  `json:"period"`
	Sales         float64 `json:"sales_cr,omitempty"`
	SalesYOY      float64 `json:"sales_yoy_pct,omitempty"`
	OperatingPft  float64 `json:"operating_profit_cr,omitempty"`
	OPM           float64 `json:"opm_pct,omitempty"`
	NetProfit     float64 `json:"net_profit_cr,omitempty"`
	NetProfitYOY  float64 `json:"net_profit_yoy_pct,omitempty"`
	EPS           float64 `json:"eps,omitempty"`
	OtherIncome   float64 `json:"other_income_cr,omitempty"`
	Interest      float64 `json:"interest_cr,omitempty"`
	Depreciation  float64 `json:"depreciation_cr,omitempty"`
	ConsecDecline int     `json:"consecutive_declines,omitempty"`
}

type qtrendResult struct {
	Symbol   string      `json:"symbol"`
	View     string      `json:"view"`
	Quarters []qtrendRow `json:"quarters"`
	Flags    []string    `json:"flags,omitempty"`
}

func newNovelQtrendCmd(flags *rootFlags) *cobra.Command {
	var flagQuarters string
	var flagView string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "qtrend <symbol>",
		Short:       "Spot whether a company's quarterly profit/sales growth is accelerating or deteriorating",
		Long:        "Use this command for a single company's multi-quarter profit/sales trend. Do NOT use it to compare different companies side by side; use 'compare' instead.",
		Example:     "  screener-pp-cli qtrend INFY --quarters 8 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "INFY"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "qtrend")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("qtrend requires a company symbol"))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			if sym == "" {
				return usageErr(fmt.Errorf("qtrend requires a company symbol"))
			}
			view := flagView
			if view == "" {
				view = "consolidated"
			}
			if view != "consolidated" && view != "standalone" {
				return usageErr(fmt.Errorf("--view must be 'consolidated' or 'standalone', got %q", view))
			}
			quarters := 8
			if flagQuarters != "" {
				n, err := strconv.Atoi(flagQuarters)
				if err != nil || n <= 0 {
					return usageErr(fmt.Errorf("--quarters must be a positive integer, got %q", flagQuarters))
				}
				quarters = n
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := computeQtrend(ctx, c, sym, view, quarters)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), res, flags)
			}
			printQtrendTable(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuarters, "quarters", "8", "Number of quarters to show (max 16)")
	cmd.Flags().StringVar(&flagView, "view", "consolidated", "Financial view: consolidated or standalone")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (unused in live mode)")
	return cmd
}

func computeQtrend(ctx context.Context, c *client.Client, sym, view string, quarters int) (qtrendResult, error) {
	res := qtrendResult{Symbol: sym, View: view, Quarters: make([]qtrendRow, 0)}
	if quarters <= 0 {
		quarters = 8
	}
	if quarters > 16 {
		quarters = 16
	}
	profileURL := "/company/" + sym + "/" + view + "/"
	data, err := c.Get(ctx, profileURL, nil)
	if err != nil {
		return res, fmt.Errorf("fetching %s: %w", sym, err)
	}
	html := string(data)
	t := parseScreenerFinTable(html, "quarters")
	rowsByLabel := map[string]map[string]float64{}
	for _, row := range t.Rows {
		rowsByLabel[strings.ToLower(normalizeFinLabel(row.Label))] = row.Values
	}
	sales := rowsByLabel["sales"]
	op := rowsByLabel["operating profit"]
	opm := rowsByLabel["opm %"]
	np := rowsByLabel["net profit"]
	eps := rowsByLabel["eps in rs"]
	other := rowsByLabel["other income"]
	interest := rowsByLabel["interest"]
	dep := rowsByLabel["depreciation"]

	periods := t.Periods
	if len(periods) > quarters {
		periods = periods[len(periods)-quarters:]
	}
	consecDeclines := 0
	for i, period := range periods {
		row := qtrendRow{Period: period}
		row.Sales = sales[period]
		row.OperatingPft = op[period]
		row.OPM = opm[period]
		row.NetProfit = np[period]
		row.EPS = eps[period]
		row.OtherIncome = other[period]
		row.Interest = interest[period]
		row.Depreciation = dep[period]
		if i >= 4 {
			prevPeriod := periods[i-4]
			if prevSales, ok := sales[prevPeriod]; ok && prevSales != 0 {
				row.SalesYOY = (row.Sales - prevSales) / prevSales * 100
			}
			if prevNP, ok := np[prevPeriod]; ok && prevNP != 0 {
				row.NetProfitYOY = (row.NetProfit - prevNP) / prevNP * 100
			}
		}
		if row.NetProfit < 0 {
			consecDeclines++
		} else {
			consecDeclines = 0
		}
		row.ConsecDecline = consecDeclines
		res.Quarters = append(res.Quarters, row)
	}
	if len(res.Quarters) >= 4 {
		last := res.Quarters[len(res.Quarters)-1]
		prev := res.Quarters[len(res.Quarters)-2]
		if last.NetProfitYOY > 0 && prev.NetProfitYOY > 0 && last.NetProfitYOY > prev.NetProfitYOY {
			res.Flags = append(res.Flags, "profit growth accelerating")
		}
		// Deceleration: growth positive but shrinking quarter over quarter,
		// or back-to-back negative quarters getting worse.
		if last.NetProfitYOY > 0 && prev.NetProfitYOY > 0 && last.NetProfitYOY < prev.NetProfitYOY {
			res.Flags = append(res.Flags, "profit growth decelerating")
		}
		if last.NetProfitYOY < 0 && prev.NetProfitYOY < 0 && last.NetProfitYOY < prev.NetProfitYOY {
			res.Flags = append(res.Flags, "profit growth deteriorating")
		}
		if last.ConsecDecline >= 2 {
			res.Flags = append(res.Flags, fmt.Sprintf("%d consecutive quarters of net loss", last.ConsecDecline))
		}
	}
	return res, nil
}

func printQtrendTable(cmd *cobra.Command, res qtrendResult) {
	if len(res.Quarters) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No quarterly data found for", res.Symbol)
		return
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "Period\tSales\tSales%\tOpPft\tNetProfit\tNP%\tEPS")
	for _, r := range res.Quarters {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Period, fnum(r.Sales), pct(r.SalesYOY), fnum(r.OperatingPft), fnum(r.NetProfit), pct(r.NetProfitYOY), fnum(r.EPS))
	}
	_ = tw.Flush()
	if len(res.Flags) > 0 {
		fmt.Fprintln(cmd.OutOrStdout())
		for _, f := range res.Flags {
			fmt.Fprintln(cmd.OutOrStdout(), "*", f)
		}
	}
}

func pct(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%+.1f%%", v)
}
