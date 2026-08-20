// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/other/screener/internal/client"

	"github.com/spf13/cobra"
)

type compareEntry struct {
	Symbol        string   `json:"symbol"`
	Name          string   `json:"name,omitempty"`
	CurrentPrice  float64  `json:"current_price,omitempty"`
	MarketCap     float64  `json:"market_cap_cr,omitempty"`
	StockPE       float64  `json:"stock_pe,omitempty"`
	BookValue     float64  `json:"book_value,omitempty"`
	DividendYield float64  `json:"dividend_yield_pct,omitempty"`
	ROCE          float64  `json:"roce_pct,omitempty"`
	ROE           float64  `json:"roe_pct,omitempty"`
	SalesQtr      float64  `json:"sales_qtr_cr,omitempty"`
	NPQtr         float64  `json:"np_qtr_cr,omitempty"`
	QtrSalesVar   float64  `json:"qtr_sales_var_pct,omitempty"`
	QtrProfitVar  float64  `json:"qtr_profit_var_pct,omitempty"`
	PromotersPct  float64  `json:"promoters_pct,omitempty"`
	FIIPct        float64  `json:"fii_pct,omitempty"`
	DIIPct        float64  `json:"dii_pct,omitempty"`
	Pros          []string `json:"pros,omitempty"`
	Cons          []string `json:"cons,omitempty"`
}

type compareResult struct {
	Compared []compareEntry `json:"compared"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagView string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "compare <symbol> [symbol...]",
		Short:       "Side-by-side fundamentals for 2-4 companies (valuation, margins, growth, shareholding)",
		Long:        "Use this command to compare two or more companies' current fundamentals side by side. Do NOT use it for a single company's quarter-over-quarter trend; use 'qtrend' instead.",
		Example:     "  screener-pp-cli compare TCS HDFCBANK --agent\n  screener-pp-cli compare TCS WIPRO HCLTECH --select name,stock_pe,roce_pct",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "TCS;HDFCBANK", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare requires at least 2 company symbols"))
			}
			if len(args) > 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare supports at most 4 company symbols, got %d", len(args)))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := flagView
			if view == "" {
				view = "consolidated"
			}
			if view != "consolidated" && view != "standalone" {
				return usageErr(fmt.Errorf("--view must be 'consolidated' or 'standalone', got %q", view))
			}

			entries := make([]compareEntry, 0, len(args))
			failures := make([]map[string]string, 0)
			for _, sym := range args {
				sym = strings.ToUpper(strings.TrimSpace(sym))
				if sym == "" {
					continue
				}
				entry, err := fetchCompareEntry(ctx, c, sym, view)
				if err != nil {
					failures = append(failures, map[string]string{"symbol": sym, "error": err.Error()})
					continue
				}
				entries = append(entries, entry)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; compared the remaining %d companies\n", len(failures), len(args), len(entries))
			}
			out := compareResult{Compared: entries}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No companies could be compared.")
				return nil
			}
			printCompareTable(cmd, entries)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagView, "view", "consolidated", "Financial view: consolidated or standalone")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (unused in live mode)")
	return cmd
}

// fetchCompareEntry pulls a company profile page and extracts the
// comparison-relevant fields from the HTML.
func fetchCompareEntry(ctx context.Context, c *client.Client, sym, view string) (compareEntry, error) {
	profileURL := "/company/" + sym + "/" + view + "/"
	data, err := c.Get(ctx, profileURL, nil)
	if err != nil {
		return compareEntry{}, fmt.Errorf("fetching %s: %w", sym, err)
	}
	html := string(data)
	entry := compareEntry{Symbol: sym}
	entry.Name = parseCompanyName(html)
	ratios := parseScreenerTopRatios(html)
	entry.CurrentPrice = ratios.CurrentPrice
	entry.MarketCap = ratios.MarketCap
	entry.StockPE = ratios.StockPE
	entry.BookValue = ratios.BookValue
	entry.DividendYield = ratios.DividendYield
	entry.ROCE = ratios.ROCE
	entry.ROE = ratios.ROE
	q := parseScreenerFinTable(html, "quarters")
	entry.SalesQtr, entry.QtrSalesVar = latestAndVar(q, "Sales")
	entry.NPQtr, entry.QtrProfitVar = latestAndVar(q, "Net Profit")
	sh := parseScreenerShareholding(html)
	entry.PromotersPct = sh.Promoters
	entry.FIIPct = sh.FIIs
	entry.DIIPct = sh.DIIs
	a := parseScreenerAnalysis(html)
	entry.Pros = a.Pros
	entry.Cons = a.Cons
	return entry, nil
}

// latestAndVar returns the most recent value for a labelled row and the
// YOY variance percentage (latest vs the value 4 periods back).
func latestAndVar(t screenerFinTable, label string) (float64, float64) {
	for _, row := range t.Rows {
		rowLabel := normalizeFinLabel(row.Label)
		if !strings.EqualFold(rowLabel, label) {
			continue
		}
		periods := t.Periods
		if len(periods) == 0 {
			return 0, 0
		}
		latestPeriod := periods[len(periods)-1]
		latest := row.Values[latestPeriod]
		if len(periods) < 4 {
			return latest, 0
		}
		yearAgoPeriod := periods[len(periods)-4]
		yearAgo := row.Values[yearAgoPeriod]
		if yearAgo == 0 {
			return latest, 0
		}
		return latest, (latest - yearAgo) / yearAgo * 100
	}
	return 0, 0
}

// normalizeFinLabel strips button/icon residue from row labels
// (e.g. "Sales +" or "Sales\u00a0+" -> "Sales").
func normalizeFinLabel(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "+")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "&nbsp;")
	return strings.TrimSpace(s)
}

// parseCompanyName extracts the h1 company name.
func parseCompanyName(html string) string {
	re := regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	m := re.FindStringSubmatch(html)
	if len(m) == 2 {
		return cleanText(m[1])
	}
	return ""
}

func printCompareTable(cmd *cobra.Command, entries []compareEntry) {
	labels := []struct {
		label string
		val   func(compareEntry) string
	}{
		{"Name", func(e compareEntry) string { return e.Name }},
		{"CMP", func(e compareEntry) string { return fnum(e.CurrentPrice) }},
		{"Mar Cap (Cr)", func(e compareEntry) string { return fnum(e.MarketCap) }},
		{"P/E", func(e compareEntry) string { return fnum(e.StockPE) }},
		{"Book Value", func(e compareEntry) string { return fnum(e.BookValue) }},
		{"Div Yld %", func(e compareEntry) string { return fnum(e.DividendYield) }},
		{"ROCE %", func(e compareEntry) string { return fnum(e.ROCE) }},
		{"ROE %", func(e compareEntry) string { return fnum(e.ROE) }},
		{"Sales Qtr", func(e compareEntry) string { return fnum(e.SalesQtr) }},
		{"NP Qtr", func(e compareEntry) string { return fnum(e.NPQtr) }},
		{"Sales Var %", func(e compareEntry) string { return fnum(e.QtrSalesVar) }},
		{"Profit Var %", func(e compareEntry) string { return fnum(e.QtrProfitVar) }},
		{"Promoters %", func(e compareEntry) string { return fnum(e.PromotersPct) }},
		{"FII %", func(e compareEntry) string { return fnum(e.FIIPct) }},
		{"DII %", func(e compareEntry) string { return fnum(e.DIIPct) }},
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "Metric")
	for _, e := range entries {
		fmt.Fprintf(tw, "\t%s", e.Symbol)
	}
	fmt.Fprintln(tw)
	for _, l := range labels {
		fmt.Fprintf(tw, "%s", l.label)
		for _, e := range entries {
			fmt.Fprintf(tw, "\t%s", l.val(e))
		}
		fmt.Fprintln(tw)
	}
	_ = tw.Flush()
}

func fnum(v float64) string {
	if v == 0 {
		return "-"
	}
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}

var _ = json.Marshal
