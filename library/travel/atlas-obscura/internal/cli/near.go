// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `near` — geo search for wonders near a place or lat,lng (hand-authored).
// Supports --radius (client-side miles filter), --category (client-side tag
// filter via place detail), --min-score, and --images.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/cliutil"
)

func newNearCmd(flags *rootFlags) *cobra.Command {
	var radius float64
	var category string
	var limit int
	var minScore int
	var images bool
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "near <place-or-latlng>",
		Short: "Find Atlas Obscura wonders near a place or coordinates",
		Long: "Find wonders near a place name or \"lat,lng\", sorted by distance (miles).\n" +
			"Place names are geocoded via Open-Meteo (no key). --radius filters by distance,\n" +
			"--category filters by each place's tags (scans detail pages, bounded by --max-scan-pages).\n" +
			"Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli near \"Paris\" --radius 5 --json\n" +
			"  atlas-obscura-pp-cli near \"48.8584,2.2945\" --limit 20\n" +
			"  atlas-obscura-pp-cli near \"Edinburgh\" --category cemeteries",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search for wonders near a point")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place name or \"lat,lng\" is required"))
			}
			if limit < 1 {
				limit = 15
			}
			if maxScanPages < 1 {
				maxScanPages = 5
			}
			if cliutil.IsDogfoodEnv() {
				maxScanPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			lat, lng, label, err := resolvePoint(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			places, scanned, capHit, err := collectNear(cmd.Context(), c, lat, lng, nearFilter{
				radius:       radius,
				category:     strings.ToLower(strings.TrimSpace(category)),
				minScore:     minScore,
				limit:        limit,
				maxScanPages: maxScanPages,
				images:       images,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if s, err := aoDB(cmd.Context()); err == nil {
				for _, p := range places {
					cachePlace(s, p)
				}
				_ = s.Close()
			}

			meta := map[string]any{
				"origin":         label,
				"lat":            lat,
				"lng":            lng,
				"scanned":        scanned,
				"max_scan_pages": maxScanPages,
			}
			if category != "" {
				meta["category"] = category
			}
			if radius > 0 {
				meta["radius_miles"] = radius
			}
			if len(places) == 0 && capHit {
				meta["note"] = fmt.Sprintf("scanned %d places without a match; widen with --radius or --max-scan-pages", scanned)
			}
			return aoEmitPlaces(cmd, flags, meta, places)
		},
	}
	cmd.Flags().Float64Var(&radius, "radius", 0, "Only include places within this many miles (0 = no limit)")
	cmd.Flags().StringVar(&category, "category", "", "Only include places tagged with this Atlas Obscura category (e.g. cemeteries)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum number of matching places to return")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Only include places with at least this interestingness score (0-10 heuristic)")
	cmd.Flags().BoolVar(&images, "images", false, "Include image URLs in output")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "Maximum result pages to scan before returning (bounds --category detail fetches)")
	return cmd
}

type nearFilter struct {
	radius       float64
	category     string
	minScore     int
	limit        int
	maxScanPages int
	images       bool
}

// collectNear pages the geo search, applies client-side radius/score/category
// filters, and bounds the scan. Returns matches, count scanned, and whether the
// scan cap was hit without filling the limit.
func collectNear(ctx context.Context, c *client.Client, lat, lng float64, f nearFilter) ([]AOPlace, int, bool, error) {
	var out []AOPlace
	scanned := 0
	capHit := true
	catFetchWarned := false
	for page := 1; page <= f.maxScanPages && len(out) < f.limit; page++ {
		resp, err := aoNear(ctx, c, lat, lng, page)
		if err != nil {
			return nil, scanned, false, err
		}
		if len(resp.Results) == 0 {
			capHit = false
			break
		}
		for _, e := range resp.Results {
			scanned++
			p := e.toPlace()
			// Enforce the radius using the API distance, else a local haversine.
			if f.radius > 0 {
				d, ok := parseDistanceMiles(p.DistanceFromQuery)
				if !ok && (p.Lat != 0 || p.Lng != 0) {
					d, ok = haversineMiles(lat, lng, p.Lat, p.Lng), true
				}
				if !ok {
					// Can't verify this place is within the radius; drop it
					// rather than silently admitting an unknown-distance result.
					continue
				}
				if d > f.radius {
					// Results are distance-sorted; once past the radius we can stop.
					capHit = false
					return out, scanned, false, nil
				}
			}
			p.Score = aoScore(p)
			if f.category != "" {
				full, err := aoFetchPlaceFull(ctx, c, p.Slug)
				if err != nil {
					// Surface category-fetch trouble once so the user knows the
					// result set may be short, instead of silently under-delivering.
					if !catFetchWarned {
						fmt.Fprintf(os.Stderr, "warning: some category lookups failed (e.g. %v); results may be incomplete\n", err)
						catFetchWarned = true
					}
					continue
				}
				p.Categories = full.Categories
				if full.Description != "" {
					p.Description = full.Description
				}
				p.Score = aoScore(p)
				if !hasCategory(p.Categories, f.category) {
					continue
				}
			}
			if f.minScore > 0 && p.Score < f.minScore {
				continue
			}
			if !f.images {
				p.ImageURL = ""
			}
			out = append(out, p)
			if len(out) >= f.limit {
				capHit = false
				break
			}
		}
		if len(resp.Results) < resp.PerPage || resp.PerPage == 0 {
			capHit = false
			break
		}
	}
	return out, scanned, capHit, nil
}

func hasCategory(cats []string, want string) bool {
	want = strings.ToLower(want)
	for _, c := range cats {
		c = strings.ToLower(c)
		if c == want || strings.Contains(c, want) {
			return true
		}
	}
	return false
}
