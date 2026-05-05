// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// compareOut is the wide-table shape: rows = field names, columns = listings.
type compareOut struct {
	Fields         []string                 `json:"fields"`
	Listings       []map[string]interface{} `json:"listings"`
	FailedLookups  []compareFailure         `json:"failed_lookups,omitempty"`
}

type compareFailure struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <url-a> <url-b> [<url-c> ...]",
		Short: "Compare 2-8 listings side-by-side.",
		Long: `Pulls each listing through the local homes cache (or the listing detail
endpoint when missing) and emits an aligned wide table: rows = field names,
columns = listings. Fields surfaced: price, beds, baths, sqft, $/sqft, lot,
year, hoa, status, school count, AVM (estimate).`,
		Example:     `  redfin-pp-cli compare /TX/Austin/123-Main/home/12345 /TX/Austin/456-Oak/home/67890 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) > 8 {
				return usageErr(fmt.Errorf("compare supports at most 8 listings (got %d)", len(args)))
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			c, _ := flags.newClient()

			out := compareOut{
				Fields: []string{"url", "price", "beds", "baths", "sqft", "ppsqft", "lot", "year", "hoa", "status", "schools", "estimate"},
			}
			for _, a := range args {
				path := extractPathFromArg(a)
				l, err := lookupListingByURL(cmd.Context(), s.DB(), c, path)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: lookup %s: %v\n", path, err)
					out.FailedLookups = append(out.FailedLookups, compareFailure{URL: path, Reason: err.Error()})
					continue
				}
				if l == nil {
					out.FailedLookups = append(out.FailedLookups, compareFailure{URL: path, Reason: "no data (live fetch unavailable and not in local store)"})
					continue
				}
				ppsqft := 0.0
				if l.Sqft > 0 {
					ppsqft = float64(l.Price) / float64(l.Sqft)
				}
				row := map[string]interface{}{
					"url":      l.URL,
					"price":    l.Price,
					"beds":     l.Beds,
					"baths":    l.Baths,
					"sqft":     l.Sqft,
					"ppsqft":   ppsqft,
					"lot":      l.LotSize,
					"year":     l.YearBuilt,
					"hoa":      l.HOA,
					"status":   l.Status,
					"schools":  len(l.Schools),
					"estimate": l.Estimate,
				}
				out.Listings = append(out.Listings, row)
			}
			if out.Listings == nil {
				out.Listings = []map[string]interface{}{}
			}
			err = printJSONFiltered(cmd.OutOrStdout(), out, flags)
			if err != nil {
				return err
			}
			// If 0 listings resolved out of N requested, exit non-zero so
			// pipeline consumers can distinguish "all 403'd" from "real
			// empty result."
			if len(out.Listings) == 0 && len(args) > 0 {
				return apiErr(fmt.Errorf("compare: 0 of %d listings resolved (all lookups failed)", len(args)))
			}
			return nil
		},
	}
	return cmd
}
