// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli xbrl-pivot --tickers A,B --concepts Revenues,NetIncome
// --quarters 8` — multi-ticker XBRL pivot resolving concept aliases.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

// xbrlConceptAliases maps canonical (LODESTAR-friendly) concept names to
// their US-GAAP XBRL element name aliases.
var xbrlConceptAliases = map[string][]string{
	"Revenues": {
		"Revenues",
		"SalesRevenueNet",
		"RevenueFromContractWithCustomerExcludingAssessedTax",
		"RevenueFromContractWithCustomerIncludingAssessedTax",
	},
	"NetIncomeLoss":      {"NetIncomeLoss", "ProfitLoss"},
	"Assets":             {"Assets"},
	"Liabilities":        {"Liabilities"},
	"StockholdersEquity": {"StockholdersEquity"},
	"OperatingCashFlow":  {"NetCashProvidedByUsedInOperatingActivities"},
	"OperatingIncome":    {"OperatingIncomeLoss"},
	"GrossProfit":        {"GrossProfit"},
	"CashAndEquivalents": {"CashAndCashEquivalentsAtCarryingValue", "CashCashEquivalentsRestrictedCashAndRestrictedCashEquivalents"},
	"EPSDiluted":         {"EarningsPerShareDiluted"},
	"EPSBasic":           {"EarningsPerShareBasic"},
	"SharesOutstanding":  {"CommonStockSharesOutstanding", "EntityCommonStockSharesOutstanding"},
}

type xbrlPivotRow struct {
	Ticker string         `json:"ticker"`
	CIK    string         `json:"cik"`
	Period string         `json:"period"`
	Fiscal string         `json:"fiscal_period"`
	Year   int            `json:"fiscal_year"`
	Values map[string]any `json:"values"`
}

func newXBRLPivotCmd(flags *rootFlags) *cobra.Command {
	var tickersArg string
	var conceptsArg string
	var quarters int

	cmd := &cobra.Command{
		Use:   "xbrl-pivot",
		Short: "Multi-ticker XBRL pivot — flat ticker×quarter×concept table with alias resolution",
		Long: `Resolve XBRL concept aliases (Revenues ↔ SalesRevenueNet ↔
RevenueFromContractWithCustomerExcludingAssessedTax) into a single canonical
column per concept. Emits flat rows with values{} per row. Concepts that
have no row for a given quarter emit null rather than 0.`,
		Example:     "  edgar-pp-cli xbrl-pivot --tickers AAPL,MSFT --concepts Revenues,NetIncomeLoss --quarters 8",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if tickersArg == "" || conceptsArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}

			var tickers []string
			for _, t := range strings.Split(tickersArg, ",") {
				if t = strings.TrimSpace(t); t != "" {
					tickers = append(tickers, t)
				}
			}
			var concepts []string
			for _, ct := range strings.Split(conceptsArg, ",") {
				if ct = strings.TrimSpace(ct); ct != "" {
					concepts = append(concepts, ct)
				}
			}
			if quarters <= 0 {
				quarters = 8
			}

			var rows []xbrlPivotRow
			for _, t := range tickers {
				ec, terr := resolveCIKOrTicker(cmd.Context(), c, db, t)
				if terr != nil {
					return classifyAPIError(terr, flags)
				}
				if err := syncCompanyFactsForCIK(cmd.Context(), c, db, ec.CIK); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"companyfacts_warning","cik":%q,"error":%q}`+"\n", ec.CIK, err.Error())
				}
				var allAliases []string
				aliasOf := map[string]string{}
				for _, canonical := range concepts {
					aliases := xbrlConceptAliases[canonical]
					if len(aliases) == 0 {
						aliases = []string{canonical}
					}
					for _, a := range aliases {
						allAliases = append(allAliases, a)
						aliasOf[a] = canonical
					}
				}
				facts, ferr := db.QueryEdgarXBRLFacts(cmd.Context(), ec.CIK, allAliases, "")
				if ferr != nil {
					return ferr
				}
				type periodKey struct {
					period string
					fp     string
					year   int
				}
				agg := map[periodKey]map[string]any{}
				for _, f := range facts {
					k := periodKey{period: f.PeriodEnd, fp: f.FiscalPeriod, year: f.FiscalYear}
					if agg[k] == nil {
						agg[k] = map[string]any{}
					}
					canonical := aliasOf[f.Concept]
					if canonical == "" {
						canonical = f.Concept
					}
					if _, exists := agg[k][canonical]; !exists {
						agg[k][canonical] = f.Value
					}
				}
				var keys []periodKey
				for k := range agg {
					keys = append(keys, k)
				}
				sort.Slice(keys, func(i, j int) bool { return keys[i].period > keys[j].period })
				if len(keys) > quarters {
					keys = keys[:quarters]
				}
				for _, k := range keys {
					values := map[string]any{}
					for _, canonical := range concepts {
						if v, ok := agg[k][canonical]; ok {
							values[canonical] = v
						} else {
							values[canonical] = nil
						}
					}
					rows = append(rows, xbrlPivotRow{
						Ticker: ec.Ticker, CIK: ec.CIK, Period: k.period, Fiscal: k.fp, Year: k.year, Values: values,
					})
				}
			}
			return emitJSON(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&tickersArg, "tickers", "", "Comma-separated tickers (or CIKs)")
	cmd.Flags().StringVar(&conceptsArg, "concepts", "", "Canonical concepts (e.g., Revenues,NetIncomeLoss,Assets)")
	cmd.Flags().IntVar(&quarters, "quarters", 8, "Number of most-recent quarters")
	return cmd
}
