// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// fxQuote is the landed-cost breakdown for one bean in the user's
// target currency.
type fxQuote struct {
	BeanLabel        string   `json:"bean"`
	OriginalPrice    int      `json:"original_price_cents"`
	OriginalCurrency string   `json:"original_currency"`
	FXRate           float64  `json:"fx_rate"`
	ConvertedPrice   int      `json:"converted_price_cents"`
	ShippingCents    int      `json:"shipping_cents,omitempty"`
	LandedPriceCents int      `json:"landed_price_cents"`
	TargetCurrency   string   `json:"target_currency"`
	PricePerOz       string   `json:"price_per_oz,omitempty"`
	Sources          []string `json:"sources"`
}

// roasterShippingRow declares a per-roaster shipping cost in the
// roaster's own currency, the destinations it ships to, and a flat
// fee in cents. Curated static reference data.
//
// pp:novel-static-reference — these numbers come from each roaster's
// public shipping page snapshot (mid-2026) and are intentionally rounded;
// they don't drive remote API calls.
type roasterShippingRow struct {
	RoasterSlug     string
	OriginCurrency  string
	DomesticUS      int
	InternationalUS int
	EUDomestic      int
	UKDomestic      int
}

var curatedRoasterShipping = []roasterShippingRow{
	// pp:novel-static-reference
	{"onyx", "USD", 800, 4500, 4500, 4500},
	{"sey", "USD", 700, 3500, 4500, 4500},
	{"glitch", "JPY", 0, 0, 0, 0}, // Japan-domestic only; international quotes case-by-case
	{"april", "DKK", 0, 0, 5500, 5500},
	{"la-cabra", "DKK", 0, 0, 5500, 5500},
	{"the-barn", "EUR", 0, 0, 6000, 6000},
	{"manhattan", "EUR", 0, 0, 6000, 6000},
	{"friedhats", "EUR", 0, 0, 6000, 6000},
	{"five-elephant", "EUR", 0, 0, 6000, 6000},
	{"square-mile", "GBP", 0, 0, 6000, 5000},
	{"workshop", "GBP", 0, 0, 6000, 5000},
	{"leaves", "JPY", 0, 0, 0, 0},
}

// curatedFXRates is a small static table of the most recent ECB / OFX
// snapshot to keep `fx` working offline. Format: rate to USD. Updated
// alongside the shipping table on Printing Press releases.
//
// pp:novel-static-reference
var curatedFXRates = map[string]float64{
	"USD": 1.00,
	"EUR": 1.08,
	"GBP": 1.27,
	"DKK": 0.145,
	"JPY": 0.0066,
	"AUD": 0.66,
	"CAD": 0.74,
}

func newFXCmd(flags *rootFlags) *cobra.Command {
	var target string
	var destination string
	cmd := &cobra.Command{
		Use:   "fx <bean>",
		Short: "Landed-cost quote: convert a roaster_products price to a target currency and add curated shipping to your destination",
		Long: `Looks up the bean's price + currency from the synced roaster_products
table, converts to --target currency at the curated rate, and adds
the curated per-roaster shipping fee for --to (us-domestic / us-intl /
eu / uk). Shipping numbers are conservative snapshots — see the source
list in --json output.`,
		Example: `  coffee-goat-pp-cli fx sey/banko-gotiti --target USD --to us-domestic --agent
  coffee-goat-pp-cli fx april/ethiopia-natural --target USD --to us-intl`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			target = strings.ToUpper(strings.TrimSpace(target))
			if target == "" {
				target = "USD"
			}
			if _, ok := curatedFXRates[target]; !ok {
				return usageErr(fmt.Errorf("unsupported target currency %q (supported: USD, EUR, GBP, DKK, JPY, AUD, CAD)", target))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			quote, err := buildFXQuote(db, args[0], target, destination)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), quote, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "fx quote for %s:\n", quote.BeanLabel)
			fmt.Fprintf(cmd.OutOrStdout(), "  list: %d %s\n", quote.OriginalPrice, quote.OriginalCurrency)
			fmt.Fprintf(cmd.OutOrStdout(), "  rate: 1 %s = %f %s\n", quote.OriginalCurrency, quote.FXRate, quote.TargetCurrency)
			fmt.Fprintf(cmd.OutOrStdout(), "  converted: %d %s\n", quote.ConvertedPrice, quote.TargetCurrency)
			if quote.ShippingCents > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  shipping (%s): %d %s\n", destination, quote.ShippingCents, quote.TargetCurrency)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  landed: %d %s  %s\n", quote.LandedPriceCents, quote.TargetCurrency, quote.PricePerOz)
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "USD", "Target currency (USD, EUR, GBP, DKK, JPY, AUD, CAD)")
	cmd.Flags().StringVar(&destination, "to", "us-domestic", "Shipping destination (us-domestic, us-intl, eu, uk)")
	return cmd
}

func buildFXQuote(db *store.Store, target, targetCurrency, destination string) (fxQuote, error) {
	roaster, handle := splitRoasterHandle(target)
	q := `SELECT COALESCE(roaster_slug,''), COALESCE(handle,''), COALESCE(title,''),
	             COALESCE(price_cents,0), COALESCE(currency,''), COALESCE(weight_g,0)
	      FROM roaster_products WHERE LOWER(handle) = LOWER(?)`
	args := []any{handle}
	if roaster != "" {
		q += ` AND LOWER(roaster_slug) = LOWER(?)`
		args = append(args, roaster)
	}
	q += ` LIMIT 1`
	var slug, hdl, title, currency string
	var price, weight int
	row := db.DB().QueryRow(q, args...)
	if err := row.Scan(&slug, &hdl, &title, &price, &currency, &weight); err == sql.ErrNoRows {
		return fxQuote{}, notFoundErr(fmt.Errorf("bean %q not found in roaster_products", target))
	} else if err != nil {
		return fxQuote{}, err
	}
	if price <= 0 || currency == "" {
		return fxQuote{}, fmt.Errorf("bean %s/%s has no priced row (price=%d currency=%q)", slug, hdl, price, currency)
	}
	currency = strings.ToUpper(currency)
	if _, ok := curatedFXRates[currency]; !ok {
		return fxQuote{}, fmt.Errorf("source currency %q not in curated FX table", currency)
	}

	// Convert via USD as pivot: cents(target) = cents(source) * rateToUSD(source) / rateToUSD(target).
	rate := curatedFXRates[currency] / curatedFXRates[targetCurrency]
	converted := int(math.Round(float64(price) * rate))
	shipping := lookupRoasterShipping(slug, destination)
	// Shipping is recorded in the roaster's currency; convert it too.
	if shipping > 0 {
		shipping = int(math.Round(float64(shipping) * rate))
	}
	landed := converted + shipping
	quote := fxQuote{
		BeanLabel:        slug + "/" + hdl + " (" + title + ")",
		OriginalPrice:    price,
		OriginalCurrency: currency,
		FXRate:           round3(rate),
		ConvertedPrice:   converted,
		ShippingCents:    shipping,
		LandedPriceCents: landed,
		TargetCurrency:   targetCurrency,
		Sources:          []string{"curated FX snapshot (pp:novel-static-reference)", "per-roaster shipping table (pp:novel-static-reference)"},
	}
	if weight > 0 {
		ozs := float64(weight) / 28.3495
		quote.PricePerOz = fmt.Sprintf("$%.2f/oz", float64(landed)/100.0/ozs)
	}
	return quote, nil
}

func lookupRoasterShipping(roasterSlug, destination string) int {
	dest := strings.ToLower(strings.TrimSpace(destination))
	for _, row := range curatedRoasterShipping {
		if row.RoasterSlug != roasterSlug {
			continue
		}
		switch dest {
		case "us-domestic", "us":
			return row.DomesticUS
		case "us-intl", "us-international", "international":
			return row.InternationalUS
		case "eu":
			return row.EUDomestic
		case "uk", "gb":
			return row.UKDomestic
		}
	}
	return 0
}
