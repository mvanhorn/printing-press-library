// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: deals whose validity window closes soon.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelExpiringCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagCategory int
	var flagWithin string
	var flagLimit int
	var flagMaxScanPages int
	var flagMaxMerchants int

	cmd := &cobra.Command{
		Use:   "expiring",
		Short: "List deals whose validity window closes within N days.",
		Long: `List a city's deals whose validity window closes within a time window.

Scans the city+category merchants, collects their deals, and keeps the ones whose
end date falls within --within (accepts 7d, 2w, 48h, etc.), soonest first. Use this
to catch use-it-or-lose-it offers. Do NOT use it to filter by open-now; use 'open-now'.`,
		Example: "  peekaboo-pp-cli expiring --city lahore --category 1 --within 7d\n  peekaboo-pp-cli expiring --city karachi --category 1 --within 14d --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--city=Lahore;--category=1;--within=30d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list deals expiring within the window")
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
			within, err := cliutil.ParseDurationLoose(flagWithin)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --within %q: %w", flagWithin, err))
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
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: merchant listing incomplete; results computed over the scanned results: %v\n", err)
			}

			now := time.Now()
			cutoff := now.Add(within)
			type expiringDeal struct {
				dealWithMerchant
				DaysLeft int       `json:"days_left"`
				endAt    time.Time `json:"-"`
			}
			matches := make([]expiringDeal, 0)
			for _, d := range deals {
				end, ok := parseDealTime(d.EndDate)
				if !ok {
					continue
				}
				if end.After(now) && !end.After(cutoff) {
					matches = append(matches, expiringDeal{
						dealWithMerchant: d,
						DaysLeft:         int(end.Sub(now).Hours() / 24),
						endAt:            end,
					})
				}
			}
			sort.SliceStable(matches, func(i, j int) bool { return matches[i].endAt.Before(matches[j].endAt) })
			if flagLimit > 0 && len(matches) > flagLimit {
				matches = matches[:flagLimit]
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d merchant deal-fetches failed; results computed over the rest\n", len(failures), scanned)
			}
			out := struct {
				City             string            `json:"city"`
				Category         int               `json:"category"`
				Within           string            `json:"within"`
				ScannedMerchants int               `json:"scanned_merchants"`
				DealCount        int               `json:"deal_count"`
				Deals            []expiringDeal    `json:"deals"`
				FetchFailures    []pkbFetchFailure `json:"fetch_failures"`
				Note             string            `json:"note,omitempty"`
			}{City: loc.City, Category: flagCategory, Within: flagWithin, ScannedMerchants: scanned, DealCount: len(matches), Deals: matches, FetchFailures: failures}
			if len(matches) == 0 {
				out.Note = fmt.Sprintf("scanned %d merchants; no deals expiring within %s (widen --within or raise --max-scan-pages)", scanned, flagWithin)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), out.Note)
				return nil
			}
			rows := make([][]string, 0, len(matches))
			for _, d := range matches {
				rows = append(rows, []string{fmt.Sprintf("%dd", d.DaysLeft), fmt.Sprintf("%d%%", d.PercentageValue), d.MerchantName, cliutil.CleanText(d.Title)})
			}
			return flags.printTable(cmd, []string{"DAYS LEFT", "DISCOUNT", "MERCHANT", "DEAL"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City name (required)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id (required; 1=Food)")
	cmd.Flags().StringVar(&flagWithin, "within", "7d", "Time window (e.g. 7d, 2w, 48h)")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max deals to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Max merchant list pages to scan")
	cmd.Flags().IntVar(&flagMaxMerchants, "max-merchants", 40, "Max merchants to fetch deals from")
	return cmd
}
