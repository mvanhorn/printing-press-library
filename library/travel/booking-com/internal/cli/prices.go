// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type priceRow struct {
	Checkin  string  `json:"checkin"`
	Checkout string  `json:"checkout"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

type destinationPriceRow struct {
	PropertyName string  `json:"property_name"`
	Slug         string  `json:"slug"`
	Country      string  `json:"country,omitempty"`
	Checkin      string  `json:"checkin"`
	Checkout     string  `json:"checkout"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
}

func newPricesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "prices",
		Short:       "Sweep Booking.com prices across date windows",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPricesCheapestCmd(flags), newPricesCheapestDestinationCmd(flags))
	return cmd
}

func newPricesCheapestCmd(flags *rootFlags) *cobra.Command {
	var slug, country, window string
	var nights, adults, limit int
	cmd := &cobra.Command{
		Use:         "cheapest",
		Short:       "Find cheapest check-in dates for one hotel",
		Example:     "  booking-com-pp-cli prices cheapest --slug auliviaopera --country fr --window 2026-06-01..2026-06-15 --nights 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]priceRow, 0))
			}
			if slug == "" || country == "" || window == "" || nights <= 0 {
				return cmd.Help()
			}
			dates, err := dateWindow(window)
			if err != nil {
				return fmt.Errorf("prices cheapest: %w", err)
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("prices cheapest: %w", err)
			}
			defer st.Close()
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("prices cheapest: %w", err)
			}
			out := make([]priceRow, 0)
			for _, d := range dates {
				checkout := d.AddDate(0, 0, nights)
				fmt.Fprintf(cmd.ErrOrStderr(), "fetching %s...\n", d.Format(dateOnly))
				data, err := c.Get(hotelPath(country, slug), hotelParams(d, checkout, adults))
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s failed: %v\n", d.Format(dateOnly), err)
					continue
				}
				prop, err := parseHotel(data)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s parse failed: %v\n", d.Format(dateOnly), err)
					continue
				}
				price, currency := hotelPrice(prop)
				if price <= 0 {
					continue
				}
				if err := insertPrice(cmd.Context(), st.DB(), slug, country, d.Format(dateOnly), checkout.Format(dateOnly), adults, currency, price); err != nil {
					return fmt.Errorf("prices cheapest: %w", err)
				}
				out = append(out, priceRow{Checkin: d.Format(dateOnly), Checkout: checkout.Format(dateOnly), Price: price, Currency: currency})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "Hotel slug")
	cmd.Flags().StringVar(&country, "country", "", "Hotel country code")
	cmd.Flags().StringVar(&window, "window", "", "Date window YYYY-MM-DD..YYYY-MM-DD")
	cmd.Flags().IntVar(&nights, "nights", 0, "Number of nights")
	cmd.Flags().IntVar(&adults, "adults", 2, "Adult guests")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum rows")
	return cmd
}

func newPricesCheapestDestinationCmd(flags *rootFlags) *cobra.Command {
	var query, window string
	var nights, limit int
	var maxPrice float64
	cmd := &cobra.Command{
		Use:         "cheapest-destination",
		Short:       "Find cheapest destination results across check-in dates",
		Example:     "  booking-com-pp-cli prices cheapest-destination --query Paris --window 2026-06-01..2026-06-10 --nights 2 --max-price 250",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]destinationPriceRow, 0))
			}
			if query == "" || window == "" || nights <= 0 || maxPrice <= 0 {
				return cmd.Help()
			}
			dates, err := dateWindow(window)
			if err != nil {
				return fmt.Errorf("prices cheapest-destination: %w", err)
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("prices cheapest-destination: %w", err)
			}
			defer st.Close()
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("prices cheapest-destination: %w", err)
			}
			out := make([]destinationPriceRow, 0)
			for _, d := range dates {
				checkout := d.AddDate(0, 0, nights)
				fmt.Fprintf(cmd.ErrOrStderr(), "fetching %s...\n", d.Format(dateOnly))
				data, err := c.Get("/searchresults.html", searchParams(query, d, checkout, 2, ""))
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s failed: %v\n", d.Format(dateOnly), err)
					continue
				}
				cards, err := parseCards(data)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s parse failed: %v\n", d.Format(dateOnly), err)
					continue
				}
				for _, card := range cards {
					if card.Price <= 0 || card.Price > maxPrice || card.Slug == "" {
						continue
					}
					_ = insertPrice(cmd.Context(), st.DB(), card.Slug, card.Country, d.Format(dateOnly), checkout.Format(dateOnly), 2, card.Currency, card.Price)
					out = append(out, destinationPriceRow{PropertyName: card.Name, Slug: card.Slug, Country: card.Country, Checkin: d.Format(dateOnly), Checkout: checkout.Format(dateOnly), Price: card.Price, Currency: card.Currency})
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Destination city")
	cmd.Flags().StringVar(&window, "window", "", "Date window YYYY-MM-DD..YYYY-MM-DD")
	cmd.Flags().IntVar(&nights, "nights", 0, "Number of nights")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum rows")
	return cmd
}
