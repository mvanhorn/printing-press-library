// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// rankRow is one ranked listing record emitted by `rank`.
type rankRow struct {
	URL     string  `json:"url"`
	Status  string  `json:"status,omitempty"`
	Beds    float64 `json:"beds,omitempty"`
	Baths   float64 `json:"baths,omitempty"`
	Sqft    int     `json:"sqft,omitempty"`
	Price   int     `json:"price"`
	HOA     int     `json:"hoa,omitempty"`
	Ratio   float64 `json:"ratio"`
	RatioBy string  `json:"ratio_by"`
	Region  int64   `json:"region_id,omitempty"`
}

func newRankCmd(flags *rootFlags) *cobra.Command {
	var by string
	var netHOA bool
	var regionsCSV string
	var regionID int64
	var regionType int
	var bedsMin float64
	var priceMax int
	var status string
	var limit int

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank synced listings by price-per-sqft, price-per-bed, or price.",
		Long: `Reads listings from the local homes table (synced via sync-search or
listing detail caches), computes a ratio per --by flag, and returns the
ascending-sorted ranking.

When --net-hoa is set, the effective price subtracts HOA × 12 × 5 from the
asking price — a 5-year HOA-cost projection that makes condo-vs-house
comparisons more honest.`,
		Example: `  redfin-pp-cli rank --by price-per-sqft --json
  redfin-pp-cli rank --by price-per-sqft --net-hoa --regions 30772:6,29470:6 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []rankRow{}, flags)
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			var listings []redfin.Listing
			switch {
			case regionsCSV != "":
				seen := map[string]bool{}
				for _, r := range strings.Split(regionsCSV, ",") {
					r = strings.TrimSpace(r)
					if r == "" {
						continue
					}
					id, typ, perr := parseRegionSlug(r)
					if perr != nil {
						return usageErr(perr)
					}
					rs, err := listingsFromHomesTable(cmd.Context(), s.DB(), id, typ)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "rank region %s: ERROR %v\n", r, err)
						continue
					}
					added := 0
					for _, l := range rs {
						if seen[l.URL] {
							continue
						}
						seen[l.URL] = true
						listings = append(listings, l)
						added++
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "rank region %s: %d listing(s)\n", r, added)
				}
			default:
				rs, err := listingsFromHomesTable(cmd.Context(), s.DB(), regionID, regionType)
				if err != nil {
					return err
				}
				listings = rs
				if regionID != 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "rank region %d: %d listing(s)\n", regionID, len(listings))
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "rank: %d listing(s) in local store (no region filter)\n", len(listings))
				}
			}

			out := []rankRow{}
			wantStatus := strings.ToLower(status)
			for _, l := range listings {
				if bedsMin > 0 && l.Beds < bedsMin {
					continue
				}
				if priceMax > 0 && l.Price > priceMax {
					continue
				}
				if wantStatus != "" && !strings.EqualFold(l.Status, wantStatus) {
					continue
				}
				price := l.Price
				if netHOA && l.HOA > 0 {
					price -= l.HOA * 12 * 5
				}
				if price <= 0 {
					continue
				}
				var ratio float64
				switch by {
				case "price-per-bed":
					if l.Beds <= 0 {
						continue
					}
					ratio = float64(price) / l.Beds
				case "price":
					ratio = float64(price)
				case "price-per-sqft", "":
					if l.Sqft <= 0 {
						continue
					}
					ratio = float64(price) / float64(l.Sqft)
				default:
					return usageErr(fmt.Errorf("invalid --by %q (one of: price-per-sqft, price-per-bed, price)", by))
				}
				out = append(out, rankRow{
					URL:     l.URL,
					Status:  l.Status,
					Beds:    l.Beds,
					Baths:   l.Baths,
					Sqft:    l.Sqft,
					Price:   l.Price,
					HOA:     l.HOA,
					Ratio:   ratio,
					RatioBy: ratioLabel(by),
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Ratio < out[j].Ratio })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if out == nil {
				out = []rankRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "price-per-sqft", "Ratio: price-per-sqft|price-per-bed|price")
	cmd.Flags().BoolVar(&netHOA, "net-hoa", false, "Subtract HOA*12*5 from price before computing ratio")
	cmd.Flags().StringVar(&regionsCSV, "regions", "", "Comma-separated region slugs/ids (overrides --region-id)")
	cmd.Flags().Int64Var(&regionID, "region-id", 0, "Single region ID (used when --regions empty)")
	cmd.Flags().IntVar(&regionType, "region-type", 0, "Single region type")
	cmd.Flags().Float64Var(&bedsMin, "beds-min", 0, "Minimum beds filter (post-fetch)")
	cmd.Flags().IntVar(&priceMax, "price-max", 0, "Maximum price filter (post-fetch)")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (Active|Sold|Pending)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return after sort.")
	return cmd
}

func ratioLabel(by string) string {
	switch by {
	case "price-per-bed":
		return "$/bed"
	case "price":
		return "$"
	}
	return "$/sqft"
}
