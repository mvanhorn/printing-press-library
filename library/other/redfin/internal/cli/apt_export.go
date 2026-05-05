// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newRedfinExportCmd(flags *rootFlags) *cobra.Command {
	var regionSlug string
	var status string
	var year int
	var format string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export Redfin gis search results, slicing price bands when needed.",
		Long: `Page-walks /stingray/api/gis-csv with sliced price bands when an initial
query exceeds 350 rows. The slice algorithm starts with the full price
range; if a band returns the page cap, it splits at the midpoint and
recurses. Bands are deduped by listing URL.`,
		Example: `  redfin-pp-cli export --region-slug "city/30772/TX/Austin" --status sold --year 2024 --csv
  redfin-pp-cli export --region-slug "city/30772/TX/Austin" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if regionSlug == "" {
				return usageErr(fmt.Errorf("--region-slug required"))
			}
			id, typ, err := parseRegionSlug(regionSlug)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			statusCode, err := statusCodeFor(status)
			if err != nil {
				return usageErr(err)
			}
			base := redfin.SearchOptions{
				RegionID:   id,
				RegionType: typ,
				Status:     statusCode,
				NumHomes:   350,
			}
			if statusCode == 7 {
				base.SoldFlags = "9"
			}
			if year > 0 {
				base.YearMin = year
				base.YearMax = year
			}

			seen := map[string]bool{}
			var collected []redfin.Listing
			bandsTotal, bandsFailed := 0, 0
			// Recursive band walk.
			var walk func(min, max int)
			walk = func(min, max int) {
				bandsTotal++
				opts := base
				opts.PriceMin = min
				opts.PriceMax = max
				params := redfin.BuildSearchParams(opts)
				data, gerr := c.Get("/stingray/api/gis", params)
				if gerr != nil {
					bandsFailed++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: band [%d,%d]: %v\n", min, max, gerr)
					return
				}
				listings, perr := redfin.ParseSearchResponse(data)
				if perr != nil {
					bandsFailed++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: parse band [%d,%d]: %v\n", min, max, perr)
					return
				}
				if len(listings) >= 350 && (max == 0 || max-min > 50000) {
					mid := 0
					if max == 0 {
						// no upper bound: split at 2x lowest non-zero price
						lo := 0
						for _, l := range listings {
							if l.Price > 0 && (lo == 0 || l.Price < lo) {
								lo = l.Price
							}
						}
						mid = lo * 2
						if mid <= min {
							mid = min + 100000
						}
						walk(min, mid)
						walk(mid, 0)
						return
					}
					mid = (min + max) / 2
					walk(min, mid)
					walk(mid+1, max)
					return
				}
				for _, l := range listings {
					if seen[l.URL] {
						continue
					}
					seen[l.URL] = true
					collected = append(collected, l)
				}
			}
			walk(0, 0)

			sort.Slice(collected, func(i, j int) bool { return collected[i].URL < collected[j].URL })
			fmt.Fprintf(cmd.ErrOrStderr(), "export: wrote %d unique listings (%d/%d bands failed)\n", len(collected), bandsFailed, bandsTotal)
			var outErr error
			if len(collected) == 0 && bandsTotal > 0 && bandsFailed == bandsTotal {
				// Every band failed — pipeline consumers must not see this as a real empty result.
				outErr = apiErr(fmt.Errorf("export: 0 of %d bands succeeded; check the warnings above", bandsTotal))
			}
			if strings.ToLower(format) == "csv" || flags.csv {
				if werr := writeCSV(cmd, collected); werr != nil {
					return werr
				}
				return outErr
			}
			if werr := printJSONFiltered(cmd.OutOrStdout(), collected, flags); werr != nil {
				return werr
			}
			return outErr
		},
	}
	cmd.Flags().StringVar(&regionSlug, "region-slug", "", "Region slug (REQUIRED)")
	cmd.Flags().StringVar(&status, "status", "for-sale", "Status: for-sale|sold|pending|coming-soon")
	cmd.Flags().IntVar(&year, "year", 0, "Filter by year built (single year)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json|csv (overrides --json/--csv root flags)")
	return cmd
}

func writeCSV(cmd *cobra.Command, listings []redfin.Listing) error {
	w := csv.NewWriter(cmd.OutOrStdout())
	defer w.Flush()
	if err := w.Write([]string{"url", "property_id", "status", "price", "beds", "baths", "sqft", "year_built", "city", "state", "postal_code"}); err != nil {
		return err
	}
	for _, l := range listings {
		row := []string{
			l.URL,
			strconv.FormatInt(l.PropertyID, 10),
			l.Status,
			strconv.Itoa(l.Price),
			strconv.FormatFloat(l.Beds, 'f', -1, 64),
			strconv.FormatFloat(l.Baths, 'f', -1, 64),
			strconv.Itoa(l.Sqft),
			strconv.Itoa(l.YearBuilt),
			l.Address.City,
			l.Address.State,
			l.Address.PostalCode,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
