// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/spf13/cobra"
)

func newListingCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout string
	var adults int
	cmd := &cobra.Command{
		Use:         "listing [listing-id]",
		Short:       "Show full detail for a listing (PDP sections)",
		Example:     "  airbnb-outreach-pp-cli listing 49070135\n  airbnb-outreach-pp-cli listing 49070135 --checkin 2026-08-10 --checkout 2026-08-14 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			c := newAirbnbClient(flags)
			data, err := c.Listing(args[0], checkin, checkout, adults)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adults")
	return cmd
}

func newQuoteCmd(flags *rootFlags) *cobra.Command {
	var p airbnb.QuoteParams
	cmd := &cobra.Command{
		Use:         "quote [listing-id]",
		Short:       "Get a read-only price breakdown for a listing and dates",
		Example:     "  airbnb-outreach-pp-cli quote 400704 --checkin 2026-08-10 --checkout 2026-08-14 --adults 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			p.ListingID = args[0]
			c := newAirbnbClient(flags)
			data, err := c.Quote(p)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().StringVar(&p.Checkin, "checkin", "", "Check-in date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&p.Checkout, "checkout", "", "Check-out date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&p.Adults, "adults", 1, "Number of adults")
	return cmd
}
