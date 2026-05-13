// Copyright 2026 ewanchen. Licensed under Apache-2.0. See LICENSE.
//
// macro snapshot — five economic-indicator calls (CPI, FFR, 10Y, UNEMPLOYMENT,
// NONFARM_PAYROLL) joined into a single card. Each indicator is cached at its
// natural TTL in av_macro_indicators; daily reruns cost 0 API calls.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type macroSnapshotResult struct {
	CPI             *macroIndicator `json:"cpi,omitempty"`
	FedFundsRate    *macroIndicator `json:"fed_funds_rate,omitempty"`
	TenYearTreasury *macroIndicator `json:"ten_year_treasury,omitempty"`
	Unemployment    *macroIndicator `json:"unemployment,omitempty"`
	NonfarmPayroll  *macroIndicator `json:"nonfarm_payroll,omitempty"`
	CacheHits       int             `json:"cache_hits"`
	FreshCalls      int             `json:"fresh_calls"`
	Warnings        []string        `json:"warnings,omitempty"`
}

type macroIndicator struct {
	Value     float64 `json:"value"`
	Date      string  `json:"date"`
	Unit      string  `json:"unit,omitempty"`
	Source    string  `json:"source"`
	FetchedAt string  `json:"fetched_at"`
}

// indicatorSpec describes one row in the indicator catalog so we can iterate
// them uniformly. ttlHours encodes the indicator-natural cache lifetime: CPI
// is monthly (~25 days), FFR is daily (~12h), UNEMPLOYMENT/NONFARM are
// monthly, TREASURY_YIELD is daily.
type indicatorSpec struct {
	name      string // av_macro_indicators.indicator
	function  string // AV function param
	extraArgs map[string]string
	ttlHours  int
}

var macroIndicatorCatalog = []indicatorSpec{
	{name: "CPI", function: "CPI", extraArgs: map[string]string{"interval": "monthly"}, ttlHours: 24 * 25},
	{name: "FEDERAL_FUNDS_RATE", function: "FEDERAL_FUNDS_RATE", extraArgs: map[string]string{"interval": "daily"}, ttlHours: 12},
	{name: "TREASURY_YIELD_10Y", function: "TREASURY_YIELD", extraArgs: map[string]string{"interval": "daily", "maturity": "10year"}, ttlHours: 12},
	{name: "UNEMPLOYMENT", function: "UNEMPLOYMENT", extraArgs: map[string]string{}, ttlHours: 24 * 25},
	{name: "NONFARM_PAYROLL", function: "NONFARM_PAYROLL", extraArgs: map[string]string{}, ttlHours: 24 * 25},
}

func newMacroCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "macro",
		Short: "US macro indicator snapshots (CPI / FFR / Treasury / Unemployment / NFP)",
	}
	cmd.AddCommand(newMacroSnapshotCmd(flags))
	return cmd
}

func newMacroSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Five-indicator macro card with indicator-natural TTL caching",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Pulls CPI, Federal Funds Rate, 10Y Treasury Yield, Unemployment, and
Nonfarm Payroll into a single card. Each indicator is cached at its natural
TTL (CPI monthly = 25 days, FFR daily = 12 hours, etc.) so daily reruns cost
0 AV calls after the first execution.

Cost: up to 5 AV calls on first run; 0 once all five are cached.`,
		Example: strings.Trim(`
  alphavantage-pp-cli macro snapshot --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := macroSnapshotResult{}
			for _, spec := range macroIndicatorCatalog {
				ind, fromCache, ferr := fetchOrLoadIndicator(cmd, c, db, spec)
				if ferr != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", spec.name, trimErrorMessage(ferr)))
					continue
				}
				if fromCache {
					result.CacheHits++
				} else {
					result.FreshCalls++
				}
				assignMacroResultField(&result, spec.name, ind)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// fetchOrLoadIndicator returns the latest value for one indicator. On cache
// hit (latest row younger than ttlHours), it doesn't hit the API at all.
func fetchOrLoadIndicator(cmd *cobra.Command, c *client.Client, db *store.Store, spec indicatorSpec) (*macroIndicator, bool, error) {
	// Check cache first: any row newer than now - ttlHours wins.
	var date string
	var value float64
	var unit string
	var fetchedAt string
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT date, value, COALESCE(unit, ''), fetched_at
		 FROM av_macro_indicators
		 WHERE indicator = ?
		   AND fetched_at >= datetime('now', ?)
		 ORDER BY date DESC LIMIT 1`,
		spec.name, fmt.Sprintf("-%d hours", spec.ttlHours),
	).Scan(&date, &value, &unit, &fetchedAt)
	if err == nil {
		return &macroIndicator{
			Value:     value,
			Date:      date,
			Unit:      unit,
			Source:    "cache",
			FetchedAt: fetchedAt,
		}, true, nil
	}

	// Cache miss — fetch.
	params := map[string]string{"function": spec.function}
	for k, v := range spec.extraArgs {
		params[k] = v
	}
	data, err := c.Get("/query", params)
	if err != nil {
		logQuotaCall(cmd, db, spec.function, "", false, err.Error())
		return nil, false, err
	}
	logQuotaCall(cmd, db, spec.function, "", true, "")

	date, value, unit, perr := parseMacroResponse(data)
	if perr != nil {
		return nil, false, perr
	}

	now := time.Now().UTC().Format(time.RFC3339)
	interval := spec.extraArgs["interval"]
	_, _ = db.DB().ExecContext(cmd.Context(),
		`INSERT OR REPLACE INTO av_macro_indicators
		 (indicator, date, value, unit, interval, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		spec.name, date, value, unit, interval, now,
	)

	return &macroIndicator{
		Value:     value,
		Date:      date,
		Unit:      unit,
		Source:    "live",
		FetchedAt: now,
	}, false, nil
}

// parseMacroResponse pulls the most-recent (date, value, unit) from the AV
// response shape, which is {"name":"...","interval":"...","unit":"...","data":[{"date":"YYYY-MM-DD","value":"N"}]}
func parseMacroResponse(data json.RawMessage) (string, float64, string, error) {
	var resp struct {
		Unit string              `json:"unit"`
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", 0, "", fmt.Errorf("parsing macro response: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", 0, resp.Unit, fmt.Errorf("macro response has no data rows")
	}
	first := resp.Data[0]
	v := parseAVFloat(first["value"])
	return first["date"], v, resp.Unit, nil
}

// assignMacroResultField routes a parsed indicator into the correct named field
// of the result struct. Keeps the JSON output stable.
func assignMacroResultField(r *macroSnapshotResult, indicator string, ind *macroIndicator) {
	switch indicator {
	case "CPI":
		r.CPI = ind
	case "FEDERAL_FUNDS_RATE":
		r.FedFundsRate = ind
	case "TREASURY_YIELD_10Y":
		r.TenYearTreasury = ind
	case "UNEMPLOYMENT":
		r.Unemployment = ind
	case "NONFARM_PAYROLL":
		r.NonfarmPayroll = ind
	}
}
