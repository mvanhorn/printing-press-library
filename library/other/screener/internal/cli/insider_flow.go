// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"text/tabwriter"

	"github.com/spf13/cobra"
)

type insiderTrade struct {
	Company    string  `json:"company"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	PersonType string  `json:"person_type,omitempty"`
	Date       string  `json:"date"`
	Type       string  `json:"type"`
	Quantity   float64 `json:"quantity"`
	Value      float64 `json:"value_lacs,omitempty"`
}

type insiderFlowRow struct {
	Symbol       string  `json:"symbol"`
	Company      string  `json:"company"`
	NetValue     float64 `json:"net_value_lacs"`
	BuyValue     float64 `json:"buy_value_lacs"`
	SellValue    float64 `json:"sell_value_lacs"`
	BuyCount     int     `json:"buy_count"`
	SellCount    int     `json:"sell_count"`
	NetDirection string  `json:"net_direction"`
}

type insiderFlowResult struct {
	SinceDays int              `json:"since_days"`
	Rows      []insiderFlowRow `json:"rows"`
}

var reTradeCompany = regexp.MustCompile(`href="(/company/[^"]+)"`)

// parseTradesTable parses the insider trades page rows. The page renders
// one <tr> per trade with field-classed <td> cells:
//   field-company_display, field-_get_insider_person_name,
//   field-_get_reporting_date, field-_get_insider_transaction_type,
//   field-_get_insider_value.
func parseTradesTable(html string) []insiderTrade {
	var out []insiderTrade
	trs := reFinRows.FindAllStringSubmatch(html, -1)
	for _, tr := range trs {
		cells := reFinCells.FindAllStringSubmatch(tr[1], -1)
		if len(cells) < 4 {
			continue
		}
		var t insiderTrade
		for _, c := range cells {
			cellHTML := c[0]
			cellText := cleanText(c[1])
			switch {
			case strings.Contains(cellHTML, "field-company_display"):
				t.Symbol = symbolFromCompanyLink(cellHTML)
				t.Company = cellText
			case strings.Contains(cellHTML, "field-_get_insider_person_name"):
				t.Name = cellText
			case strings.Contains(cellHTML, "field-_get_reporting_date"):
				t.Date = cellText
			case strings.Contains(cellHTML, "field-_get_insider_transaction_type"):
				for _, k := range []string{"Pledge Creation", "Preferential Offer", "Off Market", "Block Deal", "Bought", "Sold", "ESOP"} {
					if strings.Contains(cellText, k) {
						t.Type = k
						break
					}
				}
				qRe := regexp.MustCompile(`([\d,]+)\s*(Equity|Warrant|any other instrument)`)
				if m := qRe.FindStringSubmatch(cellText); len(m) == 3 {
					t.Quantity = parseNum(m[1])
				}
			case strings.Contains(cellHTML, "field-_get_insider_value"):
				valRe := regexp.MustCompile(`([\d,]+\.?\d*)\s*(lacs|crore|cr\.?)`)
				for _, mm := range valRe.FindAllStringSubmatch(cellText, -1) {
					v := parseNum(mm[1])
					unit := strings.ToLower(mm[2])
					if strings.HasPrefix(unit, "crore") || unit == "cr." {
						v *= 100
					}
					if v > t.Value {
						t.Value = v
					}
				}
			}
		}
		if t.Symbol != "" || t.Date != "" || t.Type != "" {
			out = append(out, t)
		}
	}
	return out
}

// symbolFromCompanyLink extracts the ticker from a /company/{SYMBOL}/ link.
func symbolFromCompanyLink(html string) string {
	if m := reTradeCompany.FindStringSubmatch(html); len(m) == 2 {
		parts := strings.Split(strings.Trim(m[1], "/"), "/")
		if len(parts) >= 2 {
			return strings.ToUpper(parts[1])
		}
	}
	return ""
}

func newNovelInsiderFlowCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagTop string
	var flagKind string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "insider-flow",
		Short:       "Answer 'who is net buying the most this month?' with per-company insider buy/sell aggregation.",
		Long:        "Use this command for pure insider-trade aggregation over a period. Do NOT use it to score a screen that happens to include an insider component; use 'rank' instead.",
		Example:     "  screener-pp-cli insider-flow --since 30d --top 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--since=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "insider-flow")
			}
			kind := flagKind
			if kind != "" && kind != "buy" && kind != "sell" && kind != "all" {
				return usageErr(fmt.Errorf("--kind must be buy, sell, or all; got %q", kind))
			}
			sinceDays := 30
			if flagSince != "" {
				d, err := parseSinceDays(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %v", err))
				}
				sinceDays = d
			}
			top := 10
			if flagTop != "" {
				n, err := strconv.Atoi(flagTop)
				if err != nil || n <= 0 {
					return usageErr(fmt.Errorf("--top must be a positive integer, got %q", flagTop))
				}
				top = n
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			pages := 2
			if sinceDays <= 0 || sinceDays > 60 {
				pages = 4
			}
			trades := make([]insiderTrade, 0)
			for p := 1; p <= pages; p++ {
				path := "/trades/insiders/"
				params := map[string]string{}
				if p > 1 {
					params["p"] = strconv.Itoa(p)
				}
				if sinceDays > 0 {
					params["reporting_date__year"] = currentYear()
				}
				data, err := getWithRateRetry(ctx, c, path, params)
				if err != nil {
					return fmt.Errorf("fetching insider trades page %d: %w", p, err)
				}
				pageTrades := parseTradesTable(string(data))
				trades = append(trades, pageTrades...)
				if len(pageTrades) == 0 {
					break
				}
			}
			type agg struct {
				symbol  string
				company string
				buyVal  float64
				sellVal float64
				buyCnt  int
				sellCnt int
			}
			aggs := map[string]*agg{}
			order := []string{}
			for _, t := range trades {
				key := t.Symbol
				if key == "" {
					key = t.Company
				}
				if key == "" {
					continue
				}
				a, ok := aggs[key]
				if !ok {
					a = &agg{symbol: t.Symbol, company: t.Company}
					aggs[key] = a
					order = append(order, key)
				}
				low := strings.ToLower(t.Type)
				if strings.Contains(low, "sold") || strings.Contains(low, "pledge") {
					a.sellVal += t.Value
					a.sellCnt++
				} else if t.Type != "" {
					a.buyVal += t.Value
					a.buyCnt++
				}
			}
			rows := make([]insiderFlowRow, 0, len(order))
			for _, key := range order {
				a := aggs[key]
				row := insiderFlowRow{
					Symbol:    a.symbol,
					Company:   a.company,
					BuyValue:  a.buyVal,
					SellValue: a.sellVal,
					BuyCount:  a.buyCnt,
					SellCount: a.sellCnt,
					NetValue:  a.buyVal - a.sellVal,
				}
				switch {
				case row.NetValue > 0:
					row.NetDirection = "buy"
				case row.NetValue < 0:
					row.NetDirection = "sell"
				default:
					row.NetDirection = "flat"
				}
				if kind == "buy" && row.NetDirection != "buy" {
					continue
				}
				if kind == "sell" && row.NetDirection != "sell" {
					continue
				}
				rows = append(rows, row)
			}
			// Sort by signed net value descending so genuine net buyers
			// rank above net sellers (the "who is net buying" question).
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].NetValue > rows[j].NetValue
			})
			if len(rows) > top {
				rows = rows[:top]
			}
			out := insiderFlowResult{SinceDays: sinceDays, Rows: rows}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no insider trades parsed — verify the session is active (run 'screener-pp-cli auth status') or widen --since")
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No insider trades found. This command needs a logged-in session — run 'screener-pp-cli auth login --chrome' first.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "Company\tNet(lacs)\tBuy(lacs)\tSell(lacs)\tBuys\tSells\tDir")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
					truncate(r.Symbol, 16), fnum(r.NetValue), fnum(r.BuyValue), fnum(r.SellValue), r.BuyCount, r.SellCount, r.NetDirection)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Look-back window (e.g. 30d, 7d, 90d; 0 = all fetched pages)")
	cmd.Flags().StringVar(&flagTop, "top", "10", "Maximum companies to show")
	cmd.Flags().StringVar(&flagKind, "kind", "all", "Filter: buy, sell, or all")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (unused in live mode)")
	return cmd
}

// parseSinceDays parses a duration like "30d", "7d", "90" (days) into days.
func parseSinceDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 30, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return n, nil
	}
	return 0, fmt.Errorf("invalid duration %q (use days like 30d)", s)
}

func currentYear() string {
	return fmt.Sprintf("%d", time.Now().Year())
}
