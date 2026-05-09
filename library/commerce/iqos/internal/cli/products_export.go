// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// ExportProduct is the normalized row type for bulk export.
type ExportProduct struct {
	Slug     string `json:"slug"`
	Country  string `json:"country"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Price    string `json:"price"`
}

func newProductsExportCmd(flags *rootFlags) *cobra.Command {
	var country string
	var format string
	var output string
	var concurrency int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the complete product catalog for one or all markets as CSV or JSON",
		Long: `Aggregate product sitemaps across one or all IQOS markets into a single
structured export. Useful for building reference datasets or feeding into
analytics pipelines.

For 'all' markets, sitemaps are fetched in parallel (use --concurrency to tune).`,
		Example: strings.Trim(`
  iqos-pp-cli products export --country us --format csv > us-catalog.csv
  iqos-pp-cli products export --country all --format json > iqos-global.json
  iqos-pp-cli products export --country gb --country de --format csv`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if country == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli products export --country <code|all>")
				return cmd.Help()
			}

			var countries []string
			if country == "all" {
				countries = allIQOSCountries()
				sort.Strings(countries)
			} else {
				// Support comma-separated list
				for _, c := range strings.Split(country, ",") {
					c = strings.TrimSpace(strings.ToLower(c))
					if c != "" {
						countries = append(countries, c)
					}
				}
			}

			if concurrency <= 0 {
				concurrency = 5
			}

			products, errs := fetchProductsParallel(cmd, countries, concurrency)

			// Report errors to stderr but continue
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "warning:", e)
			}

			// Determine output writer
			w := cmd.OutOrStdout()
			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			switch strings.ToLower(format) {
			case "csv":
				cw := csv.NewWriter(w)
				_ = cw.Write([]string{"slug", "country", "name", "url", "category", "price"})
				for _, p := range products {
					_ = cw.Write([]string{p.Slug, p.Country, p.Name, p.URL, p.Category, p.Price})
				}
				cw.Flush()
				return cw.Error()
			default: // json
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(products)
			}
		},
	}
	cmd.Flags().StringVar(&country, "country", "us", "Country code, comma-separated list, or 'all'")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.Flags().StringVar(&output, "output", "", "Write to file instead of stdout")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "Parallel sitemap fetch workers (for --country all)")
	return cmd
}

// fetchProductsParallel fetches sitemaps for multiple countries in parallel.
func fetchProductsParallel(cmd *cobra.Command, countries []string, concurrency int) ([]ExportProduct, []error) {
	type result struct {
		products []ExportProduct
		err      error
	}

	sem := make(chan struct{}, concurrency)
	results := make([]result, len(countries))
	var wg sync.WaitGroup

	for i, cc := range countries {
		i, cc := i, cc
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			lang := langForCountry(cc)
			urls, err := fetchSitemap(cmd.Context(), "https://www.iqos.com", cc, lang, "product")
			if err != nil {
				results[i] = result{err: fmt.Errorf("%s: %w", cc, err)}
				return
			}

			prods := make([]ExportProduct, 0, len(urls))
			for _, u := range urls {
				slug := extractSlugFromURL(u.Loc)
				if slug == "" {
					continue
				}
				prods = append(prods, ExportProduct{
					Slug:     slug,
					Country:  cc,
					URL:      u.Loc,
					Category: extractCategoryFromURL(u.Loc),
				})
			}
			results[i] = result{products: prods}
		}()
	}
	wg.Wait()

	var allProducts []ExportProduct
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			allProducts = append(allProducts, r.products...)
		}
	}
	return allProducts, errs
}
