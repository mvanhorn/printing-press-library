// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: card -> merchants reverse index.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelWalletCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagCategory int
	var flagLimit int
	var flagMaxScanPages int
	var flagMaxMerchants int

	cmd := &cobra.Command{
		Use:   "wallet <bank>",
		Short: "List every merchant in a city that honors a given bank card's deal (card to merchants).",
		Long: `List the merchants in a city+category that honor a given bank/card's deal.

Peekaboo only exposes merchant -> cards; this inverts it into card -> merchants by
scanning the city's merchants and their deals and keeping those whose deal source
matches <bank> (case-insensitive substring, e.g. "hbl", "meezan", "askari").

Use this for card-led planning. Do NOT use it to rank a city's deals regardless of
card; use 'top-deals'.`,
		Example: "  peekaboo-pp-cli wallet hbl --city lahore --category 1\n  peekaboo-pp-cli wallet meezan --city karachi --category 1 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "bank=hbl;--city=Lahore;--category=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find merchants honoring a bank card's deals")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("bank is required (e.g. hbl, meezan, askari)"))
			}
			if flagCity == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city is required"))
			}
			if flagCategory == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--category is required (1=Food; see 'categories list')"))
			}
			if err := ensureGuestToken(cmd.Context(), flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			bank := strings.ToLower(strings.TrimSpace(args[0]))
			loc, err := resolveCity(ctx, flags, flagCity)
			if err != nil {
				return err
			}
			maxScan, maxMerchants := scanBudget(flagMaxScanPages, flagMaxMerchants)
			deals, failures, scanned, err := fanOutCityDeals(ctx, flags, loc, flagCategory, maxScan, maxMerchants)
			if err != nil && len(deals) == 0 {
				return err
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: merchant listing incomplete; results computed over the scanned results: %v\n", err)
			}

			// Group matching deals by merchant.
			type merchantMatch struct {
				MerchantID   int                `json:"merchant_id"`
				MerchantName string             `json:"merchant_name"`
				Deals        []dealWithMerchant `json:"deals"`
				MaxDiscount  int                `json:"max_discount"`
			}
			byMerchant := map[int]*merchantMatch{}
			order := make([]int, 0)
			for _, d := range deals {
				if !strings.Contains(strings.ToLower(d.SourceEntityName), bank) {
					continue
				}
				m, ok := byMerchant[d.MerchantID]
				if !ok {
					m = &merchantMatch{MerchantID: d.MerchantID, MerchantName: d.MerchantName}
					byMerchant[d.MerchantID] = m
					order = append(order, d.MerchantID)
				}
				m.Deals = append(m.Deals, d)
				if d.PercentageValue > m.MaxDiscount {
					m.MaxDiscount = d.PercentageValue
				}
			}
			matches := make([]merchantMatch, 0, len(order))
			for _, id := range order {
				matches = append(matches, *byMerchant[id])
				if flagLimit > 0 && len(matches) >= flagLimit {
					break
				}
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d merchant deal-fetches failed; results computed over the rest\n", len(failures), scanned)
			}
			out := struct {
				Bank             string            `json:"bank"`
				City             string            `json:"city"`
				Category         int               `json:"category"`
				ScannedMerchants int               `json:"scanned_merchants"`
				MatchCount       int               `json:"match_count"`
				Merchants        []merchantMatch   `json:"merchants"`
				FetchFailures    []pkbFetchFailure `json:"fetch_failures"`
				Note             string            `json:"note,omitempty"`
			}{Bank: args[0], City: loc.City, Category: flagCategory, ScannedMerchants: scanned, MatchCount: len(matches), Merchants: matches, FetchFailures: failures}
			if len(matches) == 0 {
				out.Note = fmt.Sprintf("scanned %d merchants; none with a %q card deal (raise --max-scan-pages/--max-merchants to widen the scan)", scanned, args[0])
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), out.Note)
				return nil
			}
			rows := make([][]string, 0, len(matches))
			for _, m := range matches {
				rows = append(rows, []string{m.MerchantName, fmt.Sprintf("%d%%", m.MaxDiscount), fmt.Sprintf("%d", len(m.Deals))})
			}
			return flags.printTable(cmd, []string{"MERCHANT", "MAX DISCOUNT", "DEALS"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City name (required)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id (required; 1=Food)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max merchants to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Max merchant list pages to scan")
	cmd.Flags().IntVar(&flagMaxMerchants, "max-merchants", 40, "Max merchants to fetch deals from")
	return cmd
}
