// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/config"
	"github.com/spf13/cobra"
)

type geniusImpact struct {
	PropertyName    string  `json:"property_name"`
	Slug            string  `json:"slug"`
	PriceWithGenius float64 `json:"price_with_genius"`
	PriceWithout    float64 `json:"price_without"`
	Savings         float64 `json:"savings"`
	SavingsPct      float64 `json:"savings_pct"`
	Currency        string  `json:"currency"`
}

func newGeniusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "genius", Short: "Analyze Genius pricing impact", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newGeniusImpactCmd(flags))
	return cmd
}

func newGeniusImpactCmd(flags *rootFlags) *cobra.Command {
	var query, checkin, checkout string
	var adults int
	cmd := &cobra.Command{
		Use:         "impact",
		Short:       "Diff authenticated Genius prices against anonymous prices",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]geniusImpact, 0))
			}
			if query == "" || checkin == "" || checkout == "" {
				return cmd.Help()
			}
			in, err := time.Parse(dateOnly, checkin)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			out, err := time.Parse(dateOnly, checkout)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			authed, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			params := searchParams(query, in, out, adults, "")
			withData, err := authed.Get("/searchresults.html", params)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			anon := anonymousClient(authed, flags)
			withoutData, err := anon.Get("/searchresults.html", params)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			withCards, err := parseCards(withData)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			withoutCards, err := parseCards(withoutData)
			if err != nil {
				return fmt.Errorf("genius impact: %w", err)
			}
			anonPrices := map[string]float64{}
			for _, card := range withoutCards {
				anonPrices[firstNonEmptyString(card.Slug, card.Name)] = card.Price
			}
			outRows := make([]geniusImpact, 0)
			for _, card := range withCards {
				key := firstNonEmptyString(card.Slug, card.Name)
				base := anonPrices[key]
				if base <= 0 || card.Price <= 0 || base <= card.Price {
					continue
				}
				savings := base - card.Price
				outRows = append(outRows, geniusImpact{PropertyName: card.Name, Slug: card.Slug, PriceWithGenius: card.Price, PriceWithout: base, Savings: savings, SavingsPct: savings / base * 100, Currency: card.Currency})
			}
			return flags.printJSON(cmd, outRows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Destination city")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date YYYY-MM-DD")
	cmd.Flags().IntVar(&adults, "adults", 2, "Adult guests")
	return cmd
}

func anonymousClient(c *client.Client, flags *rootFlags) *client.Client {
	cfg := config.Config{BaseURL: c.BaseURL, Headers: map[string]string{}}
	if c.Config != nil {
		cfg = *c.Config
		cfg.AuthHeaderVal, cfg.AccessToken, cfg.RefreshToken = "", "", ""
		cfg.Headers = map[string]string{}
		for k, v := range c.Config.Headers {
			if k != "Cookie" && k != "Authorization" {
				cfg.Headers[k] = v
			}
		}
	}
	return client.New(&cfg, flags.timeout, flags.rateLimit)
}
