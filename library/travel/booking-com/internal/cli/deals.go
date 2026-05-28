// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type mobileRate struct {
	PropertyName string  `json:"property_name"`
	Slug         string  `json:"slug"`
	DesktopPrice float64 `json:"desktop_price"`
	MobilePrice  float64 `json:"mobile_price"`
	Savings      float64 `json:"savings"`
	Currency     string  `json:"currency"`
}

func newDealsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "deals", Short: "Analyze hidden Booking.com deal classes", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newDealsMobileRatesCmd(flags))
	return cmd
}

func newDealsMobileRatesCmd(flags *rootFlags) *cobra.Command {
	var query, checkin, checkout string
	cmd := &cobra.Command{
		Use:         "mobile-rates",
		Short:       "Diff desktop prices against mobile user-agent prices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]mobileRate, 0))
			}
			if query == "" || checkin == "" || checkout == "" {
				return cmd.Help()
			}
			in, err := time.Parse(dateOnly, checkin)
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			out, err := time.Parse(dateOnly, checkout)
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			params := searchParams(query, in, out, 2, "")
			desktopData, err := c.Get("/searchresults.html", params)
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			mobileUA := "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1"
			mobileData, err := c.GetWithHeadersNoCache("/searchresults.html", params, map[string]string{"User-Agent": mobileUA})
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			desktop, err := parseCards(desktopData)
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			mobile, err := parseCards(mobileData)
			if err != nil {
				return fmt.Errorf("deals mobile-rates: %w", err)
			}
			desktopPrices := map[string]float64{}
			for _, card := range desktop {
				desktopPrices[firstNonEmptyString(card.Slug, card.Name)] = card.Price
			}
			outRows := make([]mobileRate, 0)
			for _, card := range mobile {
				base := desktopPrices[firstNonEmptyString(card.Slug, card.Name)]
				if base > card.Price && card.Price > 0 {
					outRows = append(outRows, mobileRate{PropertyName: card.Name, Slug: card.Slug, DesktopPrice: base, MobilePrice: card.Price, Savings: base - card.Price, Currency: card.Currency})
				}
			}
			return flags.printJSON(cmd, outRows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Destination city")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date YYYY-MM-DD")
	return cmd
}
