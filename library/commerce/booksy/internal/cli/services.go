// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: services — flatten a business profile into bookable rows.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelServicesCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagBookableOnly bool

	cmd := &cobra.Command{
		Use:   "services <business_id>",
		Short: "Flatten a business profile into a table of bookable services with price, duration, and service-variant id",
		Long: "Flatten a Booksy business profile into a flat table of bookable services.\n" +
			"Each row shows category, service name, price, duration, and the\n" +
			"service-variant id you pass to `availability`, `earliest`, and `book`.\n" +
			"Public — no token required. Use --query to filter (e.g. --query haircut).",
		Example:     "  booksy-pp-cli services 297360 --query haircut",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "business_id=297360"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "services")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("business_id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			biz, err := fetchBusiness(ctx, c, args[0])
			if err != nil {
				return err
			}
			rows := flattenServices(biz, flagQuery)
			if flagBookableOnly {
				filtered := make([]bkServiceRow, 0, len(rows))
				for _, r := range rows {
					if r.Duration > 0 {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
			}
			view := struct {
				Business string         `json:"business"`
				BizID    int64          `json:"business_id"`
				Query    string         `json:"query,omitempty"`
				Count    int            `json:"count"`
				Services []bkServiceRow `json:"services"`
			}{Business: biz.Name, BizID: biz.ID, Query: flagQuery, Count: len(rows), Services: rows}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s (id %d)\n", biz.Name, biz.ID)
			if len(rows) == 0 {
				fmt.Fprintln(out, "No matching services found.")
				return nil
			}
			fmt.Fprintf(out, "%-12s  %-42s  %12s  %4s\n", "VARIANT-ID", "SERVICE", "PRICE", "MIN")
			for _, r := range rows {
				name := r.Service
				if len([]rune(name)) > 42 {
					name = string([]rune(name)[:41]) + "…"
				}
				fmt.Fprintf(out, "%-12d  %-42s  %12s  %4d\n", r.VariantID, name, r.PriceLabel, r.Duration)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Filter services by name (e.g. haircut, strzyżenie, beard)")
	cmd.Flags().BoolVar(&flagBookableOnly, "bookable-only", false, "Only rows with a real duration (bookable variants)")
	return cmd
}
