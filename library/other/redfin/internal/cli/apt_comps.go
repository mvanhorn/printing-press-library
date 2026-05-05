// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// compRow is one ranked comparable record.
type compRow struct {
	URL      string  `json:"url"`
	Price    int     `json:"price"`
	Sqft     int     `json:"sqft,omitempty"`
	Beds     float64 `json:"beds,omitempty"`
	Baths    float64 `json:"baths,omitempty"`
	SoldAt   string  `json:"sold_at,omitempty"`
	PPSqft   float64 `json:"price_per_sqft"`
	Distance float64 `json:"approx_miles"`
}

// circularPolygon returns a 12-vertex closed polygon string approximating a
// circle of `radiusMi` miles around (lat, lng). Stingray expects the polygon
// in "lat lng,lat lng,...,lat lng" form (closed; first vertex repeated last).
//
// dlat = miles / 69; dlng = miles / (69 * cos(lat)) — the standard small-area
// flat-earth approximation.
func circularPolygon(lat, lng, radiusMi float64) string {
	if radiusMi <= 0 {
		radiusMi = 0.5
	}
	const verts = 12
	dlat := radiusMi / 69
	dlng := radiusMi / (69 * math.Cos(lat*math.Pi/180))
	parts := make([]string, 0, verts+1)
	for i := 0; i < verts; i++ {
		theta := float64(i) * 2 * math.Pi / float64(verts)
		plat := lat + dlat*math.Sin(theta)
		plng := lng + dlng*math.Cos(theta)
		parts = append(parts, fmt.Sprintf("%.6f %.6f", plat, plng))
	}
	parts = append(parts, parts[0]) // close
	return strings.Join(parts, ",")
}

// haversineMiles returns the great-circle distance between two lat/lng pairs
// in miles. Used to score comps by proximity.
func haversineMiles(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 3958.7613 // earth radius in miles
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dlat := rad(lat2 - lat1)
	dlng := rad(lng2 - lng1)
	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func newCompsCmd(flags *rootFlags) *cobra.Command {
	var radius float64
	var sqftTol int
	var months int
	var bedMatch bool

	cmd := &cobra.Command{
		Use:   "comps <subject-url>",
		Short: "Find sold comparables near a subject listing.",
		Long: `Resolves the subject's lat/lng (from local cache or the listing
endpoint), draws an approximately-circular --radius polygon around it, runs
a sold gis search inside, and filters in-process by sqft tolerance, optional
bedroom match, and a months window.`,
		Example:     `  redfin-pp-cli comps /TX/Austin/123-Main/home/12345 --radius 0.5 --sqft-tol 15 --months 6 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (sold comps within "+fmt.Sprintf("%.2f", radius)+" miles)")
					return nil
				}
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (sold comps for "+args[0]+")")
				return nil
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			path := extractPathFromArg(args[0])
			subj, err := lookupListingByURL(cmd.Context(), s.DB(), c, path)
			if err != nil || subj == nil {
				return apiErr(fmt.Errorf("could not resolve subject listing %s", path))
			}
			if subj.Address.Latitude == 0 || subj.Address.Longitude == 0 {
				return apiErr(fmt.Errorf("subject listing has no lat/lng; cannot derive comp polygon"))
			}
			poly := circularPolygon(subj.Address.Latitude, subj.Address.Longitude, radius)
			opts := redfin.SearchOptions{
				Status:    7,
				SoldFlags: soldFlagsForMonths(months),
				Polygon:   poly,
				NumHomes:  100,
			}
			params := redfin.BuildSearchParams(opts)
			data, gerr := c.Get("/stingray/api/gis", params)
			if gerr != nil {
				return classifyAPIError(gerr)
			}
			listings, perr := redfin.ParseSearchResponse(data)
			if perr != nil {
				return apiErr(perr)
			}
			tol := float64(sqftTol)
			if tol <= 0 {
				tol = 15
			}
			tol /= 100
			rows := []compRow{}
			for _, l := range listings {
				if l.URL == subj.URL {
					continue
				}
				if subj.Sqft > 0 && l.Sqft > 0 {
					ratio := math.Abs(float64(l.Sqft-subj.Sqft) / float64(subj.Sqft))
					if ratio > tol {
						continue
					}
				}
				if bedMatch && subj.Beds > 0 && l.Beds != subj.Beds {
					continue
				}
				if l.Price <= 0 || l.Sqft <= 0 {
					continue
				}
				ppsqft := float64(l.Price) / float64(l.Sqft)
				dist := 0.0
				if l.Address.Latitude != 0 && l.Address.Longitude != 0 {
					dist = haversineMiles(subj.Address.Latitude, subj.Address.Longitude, l.Address.Latitude, l.Address.Longitude)
				}
				rows = append(rows, compRow{
					URL:      l.URL,
					Price:    l.Price,
					Sqft:     l.Sqft,
					Beds:     l.Beds,
					Baths:    l.Baths,
					SoldAt:   l.SoldAt,
					PPSqft:   ppsqft,
					Distance: dist,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Distance < rows[j].Distance })
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().Float64Var(&radius, "radius", 0.5, "Search radius in miles")
	cmd.Flags().IntVar(&sqftTol, "sqft-tol", 15, "Sqft tolerance percent (±N%)")
	cmd.Flags().IntVar(&months, "months", 6, "Sold-within window in months (1, 3, 6, 12, 24)")
	cmd.Flags().BoolVar(&bedMatch, "bed-match", false, "Require exact bed-count match")
	return cmd
}

// soldFlagsForMonths picks the closest Stingray "sf" sold-time window flag.
// Codes (from Stingray's gis): 1=1mo, 3=3mo, 5=6mo, 7=1yr, 9=2yr.
func soldFlagsForMonths(m int) string {
	switch {
	case m <= 1:
		return "1"
	case m <= 3:
		return "3"
	case m <= 6:
		return "5"
	case m <= 12:
		return "7"
	}
	return "9"
}
