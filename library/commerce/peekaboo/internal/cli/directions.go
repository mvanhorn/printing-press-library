// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: bulk Google Maps directions for every branch.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelDirectionsCmd(flags *rootFlags) *cobra.Command {
	var flagCity string
	var flagCategory int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "directions <merchant>",
		Short: "Get a ready-to-open Google Maps directions link for every branch of a restaurant at once.",
		Long: `Get a ready-to-open Google Maps directions link for every branch of a restaurant in a city.

<merchant> is a numeric entity id (from 'peekaboo-pp-cli places list'), or a name
when --category is also given so it can be resolved. Each branch is returned with
its address, coordinates, and a maps.google.com directions URL — the same link the
Peekaboo site's per-branch "Direction" button opens, but for every branch at once.

Use this command to list or export directions for all branches of a merchant. Do
NOT use it to pick only the single closest branch to a location; use 'nearest'.`,
		Example: "  peekaboo-pp-cli directions 13 --city lahore\n  peekaboo-pp-cli directions kababjees --city lahore --category 1 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "merchant=13;--city=Lahore",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch branches and build Google Maps directions URLs")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("merchant is required (a numeric entity id or a name with --category)"))
			}
			if flagCity == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--city is required"))
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
			entityID, merchantName, err := resolveEntity(ctx, flags, args[0], loc, flagCategory, 5)
			if err != nil {
				return err
			}
			branches, apiName, err := fetchBranches(ctx, flags, fmt.Sprintf("%d", entityID), loc, flagLimit)
			if err != nil {
				return err
			}
			if apiName != "" {
				merchantName = apiName
			}

			type branchDirection struct {
				pkbBranch
				DirectionsURL string `json:"directions_url"`
			}
			out := struct {
				Merchant      string            `json:"merchant"`
				MerchantID    int               `json:"merchant_id"`
				City          string            `json:"city"`
				TotalBranches int               `json:"total_branches"`
				Branches      []branchDirection `json:"branches"`
			}{Merchant: merchantName, MerchantID: entityID, City: loc.City}
			out.Branches = make([]branchDirection, 0, len(branches))
			for _, b := range branches {
				out.Branches = append(out.Branches, branchDirection{
					pkbBranch:     b,
					DirectionsURL: mapsDirectionsURL(b.Latitude, b.Longitude),
				})
			}
			out.TotalBranches = len(out.Branches)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, out)
			}
			if len(out.Branches) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No branches found for %s in %s.\n", merchantName, loc.City)
				return nil
			}
			rows := make([][]string, 0, len(out.Branches))
			for _, b := range out.Branches {
				rows = append(rows, []string{b.Name, b.Address, b.DirectionsURL})
			}
			return flags.printTable(cmd, []string{"BRANCH", "ADDRESS", "DIRECTIONS"}, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "City to look up branches in (required)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id, only needed to resolve a merchant by name (1=Food)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max branches to return")
	return cmd
}
