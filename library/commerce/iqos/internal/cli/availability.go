// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// AvailabilityResult shows which countries have a given product.
type AvailabilityResult struct {
	Slug      string   `json:"slug"`
	Available []string `json:"available"`
	Missing   []string `json:"missing"`
	Name      string   `json:"name,omitempty"`
	URL       string   `json:"url,omitempty"`
}

func newAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var sku string
	var checkCountries string

	cmd := &cobra.Command{
		Use:   "availability [sku]",
		Short: "Check which countries have a specific IQOS product",
		Long: `Look up which synced markets carry a given product slug.
Compares against all synced countries by default.

Requires synced data: iqos-pp-cli sync --country us && iqos-pp-cli sync --country gb`,
		Example: strings.Trim(`
  iqos-pp-cli availability terea-turquoise --json
  iqos-pp-cli availability --sku heets-amber --json
  iqos-pp-cli availability terea-turquoise --countries us,gb,de --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if sku == "" && len(args) > 0 {
				sku = strings.TrimSpace(args[0])
			}
			if dryRunOK(flags) {
				return nil
			}
			if sku == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli availability <sku>")
				return cmd.Help()
			}

			db, err := iqosOpenDB(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rawDB := db.DB()

			// Get distinct synced countries
			var allCountries []string
			if checkCountries != "" {
				for _, c := range strings.Split(checkCountries, ",") {
					c = strings.TrimSpace(c)
					if c != "" {
						allCountries = append(allCountries, c)
					}
				}
			} else {
				rows, err := rawDB.QueryContext(cmd.Context(),
					`SELECT DISTINCT country FROM iqos_products ORDER BY country`)
				if err != nil {
					return fmt.Errorf("listing countries: %w", err)
				}
				defer rows.Close()
				for rows.Next() {
					var cc string
					if err := rows.Scan(&cc); err == nil {
						allCountries = append(allCountries, cc)
					}
				}
			}

			if len(allCountries) == 0 {
				return fmt.Errorf("no synced data found — run 'iqos-pp-cli sync --country us' first")
			}

			// Find which countries have the product
			availableSet := make(map[string]bool)
			var productName, productURL string
			rows2, err := rawDB.QueryContext(cmd.Context(),
				`SELECT country, COALESCE(name,''), COALESCE(url,'') FROM iqos_products WHERE slug=?`,
				sku)
			if err != nil {
				return fmt.Errorf("querying product: %w", err)
			}
			defer rows2.Close()
			for rows2.Next() {
				var cc, nm, u string
				if err := rows2.Scan(&cc, &nm, &u); err == nil {
					availableSet[cc] = true
					if productName == "" {
						productName = nm
					}
					if productURL == "" {
						productURL = u
					}
				}
			}

			result := AvailabilityResult{
				Slug: sku,
				Name: productName,
				URL:  productURL,
			}
			for _, cc := range allCountries {
				if availableSet[cc] {
					result.Available = append(result.Available, cc)
				} else {
					result.Missing = append(result.Missing, cc)
				}
			}
			if result.Available == nil {
				result.Available = []string{}
			}
			if result.Missing == nil {
				result.Missing = []string{}
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().StringVar(&sku, "sku", "", "Product slug to check (e.g. terea-turquoise)")
	cmd.Flags().StringVar(&checkCountries, "countries", "", "Comma-separated country codes to check (default: all synced)")
	return cmd
}
