// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: portfolio history. Reads the local portfolio_snapshots time
// series (captured by 'brief' and 'portfolio show') — Robinhood's MCP has no
// portfolio-history endpoint, so this local series is the only record of past
// value. Store-only: no network calls.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

// portfolioHistoryEmptyNote is emitted (JSON "note" / human line) when the
// local series has no points yet.
const portfolioHistoryEmptyNote = "no portfolio snapshots yet — run 'brief' or 'portfolio show' to start capturing"

// portfolioHistoryPoint is one snapshot in the JSON output.
type portfolioHistoryPoint struct {
	CapturedAt   string `json:"captured_at"`
	TotalValue   string `json:"total_value"`
	EquityValue  string `json:"equity_value"`
	OptionsValue string `json:"options_value"`
	Cash         string `json:"cash"`
	BuyingPower  string `json:"buying_power"`
}

// snapshotTotals extracts the total-value series as floats (oldest first),
// tolerating malformed money strings as 0 via parseMoney.
// snapshotAccounts returns the distinct account numbers in the series, sorted.
func snapshotAccounts(snaps []store.PortfolioSnapshot) []string {
	seen := map[string]bool{}
	var accounts []string
	for _, s := range snaps {
		if !seen[s.AccountNumber] {
			seen[s.AccountNumber] = true
			accounts = append(accounts, s.AccountNumber)
		}
	}
	sort.Strings(accounts)
	return accounts
}

func snapshotTotals(snaps []store.PortfolioSnapshot) []float64 {
	out := make([]float64, len(snaps))
	for i, s := range snaps {
		out[i] = parseMoney(s.TotalValue)
	}
	return out
}

// summarizePortfolioHistory computes the first and last total values of an
// oldest-first snapshot series plus the absolute and percentage change between
// them. changePct is 0 when the series is empty or the first value is zero
// (no meaningful base for a percentage).
func summarizePortfolioHistory(snaps []store.PortfolioSnapshot) (first, last, change, changePct float64) {
	if len(snaps) == 0 {
		return 0, 0, 0, 0
	}
	first = parseMoney(snaps[0].TotalValue)
	last = parseMoney(snaps[len(snaps)-1].TotalValue)
	change = last - first
	if first != 0 {
		changePct = change / first * 100
	}
	return first, last, change, changePct
}

func newNovelPortfolioHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagSparkline bool
	var flagAccount string

	cmd := &cobra.Command{
		Use:         "history",
		Short:       "Answer 'what was my portfolio worth on any given day' from a local time series Robinhood doesn't keep.",
		Example:     "--since 30d --sparkline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			since, err := parseSince(flagSince)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}

			// Read-only open: history only reads the local snapshot series.
			// A nil store means no local DB exists yet — an honest empty series.
			st, err := openStoreForRead(cmd.Context(), "robinhood-agentic-pp-cli")
			if err != nil {
				return err
			}
			var snaps []store.PortfolioSnapshot
			if st != nil {
				defer st.Close()
				snaps, err = st.PortfolioSnapshots(flagAccount, since)
				if err != nil {
					return err
				}
			}
			// A summary/sparkline across accounts compares unrelated values
			// (account A's Tuesday against account B's Wednesday) — refuse
			// rather than emit a meaningless change figure.
			if flagAccount == "" {
				if accounts := snapshotAccounts(snaps); len(accounts) > 1 {
					return usageErr(fmt.Errorf("snapshots span %d accounts (%s); pass --account to pick one", len(accounts), strings.Join(accounts, ", ")))
				}
			}

			out := cmd.OutOrStdout()
			machine := flags.asJSON || !isTerminal(out)

			if len(snaps) == 0 {
				if machine {
					return printJSONFiltered(out, map[string]any{
						"snapshots": []portfolioHistoryPoint{},
						"note":      portfolioHistoryEmptyNote,
					}, flags)
				}
				fmt.Fprintln(out, portfolioHistoryEmptyNote)
				return nil
			}

			first, last, change, changePct := summarizePortfolioHistory(snaps)
			points := make([]portfolioHistoryPoint, 0, len(snaps))
			for _, s := range snaps {
				points = append(points, portfolioHistoryPoint{
					CapturedAt:   s.CapturedAt.UTC().Format(time.RFC3339),
					TotalValue:   s.TotalValue,
					EquityValue:  s.EquityValue,
					OptionsValue: s.OptionsValue,
					Cash:         s.Cash,
					BuyingPower:  s.BuyingPower,
				})
			}
			var spark string
			if flagSparkline {
				spark = renderSparkline(snapshotTotals(snaps))
			}

			if machine {
				result := map[string]any{
					"account":     flagAccount,
					"since":       since.UTC().Format(time.RFC3339),
					"count":       len(snaps),
					"first_value": first,
					"last_value":  last,
					"change":      change,
					"change_pct":  changePct,
					"snapshots":   points,
				}
				if flagSparkline {
					result["sparkline"] = spark
				}
				return printJSONFiltered(out, result, flags)
			}

			accountLabel := flagAccount
			if accountLabel == "" {
				accountLabel = "all accounts"
			}
			fmt.Fprintf(out, "Portfolio history: %d snapshots since %s (%s)\n", len(snaps), since.Format("2006-01-02"), accountLabel)
			for _, s := range snaps {
				acctCol := ""
				if flagAccount == "" {
					acctCol = "  " + s.AccountNumber
				}
				fmt.Fprintf(out, "  %s%s  total %12s  equity %12s  options %12s  cash %12s\n",
					s.CapturedAt.Local().Format("2006-01-02 15:04"), acctCol,
					s.TotalValue, s.EquityValue, s.OptionsValue, s.Cash)
			}
			if flagSparkline {
				fmt.Fprintf(out, "\n%s\n", spark)
			}
			fmt.Fprintf(out, "%.2f → %.2f  change %+.2f (%+.2f%%)\n", first, last, change, changePct)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Lower bound for snapshots: 30d, 12h, 2w, or 2026-07-01 (default 30d)")
	cmd.Flags().BoolVar(&flagSparkline, "sparkline", false, "Render the total-value series as a compact sparkline")
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to one account number (default: all accounts)")
	return cmd
}
