// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: merchants open right now.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/peekaboo/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelOpenNowCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagCategory int
	var flagLimit int
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "open-now",
		Short: "Keep only merchants with a branch open at the current local time.",
		Long: `List merchants in a city+category that are open right now.

Scans the city's merchant listing and keeps only those Peekaboo reports as open
at the current time. Use this for "where can I go right now" queries. Do NOT use
it to find deals expiring soon; use 'expiring'.`,
		Example: "  peekaboo-pp-cli open-now --city lahore --category 1\n  peekaboo-pp-cli open-now --city karachi --category 1 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--city=Lahore;--category=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list merchants and filter to those open now")
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
			maxScan := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxScan > 1 {
				maxScan = 1
			}
			entities, scanned, err := listCityEntities(ctx, flags, loc, flagCategory, maxScan, 50)
			if err != nil && len(entities) == 0 {
				return err
			}
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: merchant listing incomplete; results computed over the scanned results: %v\n", err)
			}
			open := make([]pkbEntity, 0)
			for _, e := range entities {
				if e.OpenNow {
					open = append(open, e)
					if flagLimit > 0 && len(open) >= flagLimit {
						break
					}
				}
			}
			out := struct {
				City             string      `json:"city"`
				Category         int         `json:"category"`
				ScannedMerchants int         `json:"scanned_merchants"`
				OpenCount        int         `json:"open_count"`
				Merchants        []pkbEntity `json:"merchants"`
				Note             string      `json:"note,omitempty"`
			}{City: loc.City, Category: flagCategory, ScannedMerchants: scanned, OpenCount: len(open), Merchants: open}
			if len(open) == 0 {
				out.Note = fmt.Sprintf("scanned %d merchants; none open now (raise --max-scan-pages to widen the scan)", scanned)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(open) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), out.Note)
				return nil
			}
			rows := make([][]string, 0, len(open))
			for _, e := range open {
				rows = append(rows, []string{fmt.Sprintf("%d", e.ID), e.Name, fmt.Sprintf("%d", e.Stats.Branches)})
			}
			return flags.printTable(cmd, []string{"ID", "MERCHANT", "BRANCHES"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City name (required)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id (required; 1=Food)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max merchants to return")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 3, "Max merchant list pages to scan")
	return cmd
}
