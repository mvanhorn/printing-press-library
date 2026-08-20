// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: rank a city's deals by discount percentage.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelTopDealsCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagCategory int
	var flagMinDiscount int
	var flagLimit int
	var flagMaxScanPages int
	var flagMaxMerchants int

	cmd := &cobra.Command{
		Use:   "top-deals",
		Short: "Rank a city's merchants by discount percentage and return the biggest current discounts.",
		Long: `Rank a whole city's deals by discount percentage and return the biggest ones.

Scans the city+category merchant listing, fetches each merchant's deals, and
sorts them by discount percentage. Use this to find the single biggest valid
discount in a city. Do NOT use it to filter by a specific bank card; use 'wallet'.`,
		Example: "  peekaboo-pp-cli top-deals --city lahore --category 1 --min-discount 30 --limit 10\n  peekaboo-pp-cli top-deals --city karachi --category 1 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--city=Lahore;--category=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank a city's deals by discount percentage")
				return nil
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
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: merchant listing incomplete; ranking computed over the scanned results: %v\n", err)
			}
			filtered := make([]dealWithMerchant, 0, len(deals))
			for _, d := range deals {
				if d.PercentageValue >= flagMinDiscount {
					filtered = append(filtered, d)
				}
			}
			sortDealsByDiscountDesc(filtered)
			if flagLimit > 0 && len(filtered) > flagLimit {
				filtered = filtered[:flagLimit]
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d merchant deal-fetches failed; ranking computed over the rest\n", len(failures), scanned)
			}
			out := struct {
				City             string             `json:"city"`
				Category         int                `json:"category"`
				MinDiscount      int                `json:"min_discount"`
				ScannedMerchants int                `json:"scanned_merchants"`
				DealCount        int                `json:"deal_count"`
				Deals            []dealWithMerchant `json:"deals"`
				FetchFailures    []pkbFetchFailure  `json:"fetch_failures"`
				Note             string             `json:"note,omitempty"`
			}{City: loc.City, Category: flagCategory, MinDiscount: flagMinDiscount, ScannedMerchants: scanned, DealCount: len(filtered), Deals: filtered, FetchFailures: failures}
			if len(filtered) == 0 {
				out.Note = fmt.Sprintf("scanned %d merchants; no deals at or above %d%% (lower --min-discount or raise --max-scan-pages)", scanned, flagMinDiscount)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(filtered) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), out.Note)
				return nil
			}
			rows := make([][]string, 0, len(filtered))
			for _, d := range filtered {
				rows = append(rows, []string{fmt.Sprintf("%d%%", d.PercentageValue), d.MerchantName, cliutil.CleanText(d.Title)})
			}
			return flags.printTable(cmd, []string{"DISCOUNT", "MERCHANT", "DEAL"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City name (required)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id (required; 1=Food)")
	cmd.Flags().IntVar(&flagMinDiscount, "min-discount", 0, "Only include deals at or above this discount percentage")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max deals to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Max merchant list pages to scan")
	cmd.Flags().IntVar(&flagMaxMerchants, "max-merchants", 40, "Max merchants to fetch deals from")
	return cmd
}

// scanBudget curtails the fan-out width under live dogfood so the matrix's
// per-command timeout is respected, while leaving interactive runs generous.
func scanBudget(maxScanPages, maxMerchants int) (int, int) {
	if cliutil.IsDogfoodEnv() {
		if maxScanPages > 1 {
			maxScanPages = 1
		}
		if maxMerchants > 6 {
			maxMerchants = 6
		}
	}
	return maxScanPages, maxMerchants
}
