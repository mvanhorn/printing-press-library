// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel transcendence command: closest branch of a merchant to a point.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelNearestCmd(flags *rootFlags) *cobra.Command {
	var flagNear string
	var flagCity string
	var flagCategory int

	cmd := &cobra.Command{
		Use:   "nearest <merchant>",
		Short: "Find the single closest branch of a restaurant to a city or coordinates, with its directions link.",
		Long: `Find the single closest branch of a restaurant to a reference point.

<merchant> is a numeric entity id (from 'peekaboo-pp-cli places list'), or a name
when --category is also given. --near accepts a city name (branches are scoped to
that city and distance is measured from its center) or raw "lat,long" coordinates
(pass --city too so branches can be scoped). The closest branch is returned with
its distance and a Google Maps directions URL.

Use this command to find the single closest branch. Do NOT use it to list all
branches; use 'directions'.`,
		Example: "  peekaboo-pp-cli nearest 13 --near lahore\n  peekaboo-pp-cli nearest 13 --near 31.55,74.35 --city lahore --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "merchant=13;--near=Lahore",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch branches and compute the nearest one")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("merchant is required (a numeric entity id or a name with --category)"))
			}
			if flagNear == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--near is required (a city name or \"lat,long\")"))
			}
			if err := ensureGuestToken(cmd.Context(), flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var refLat, refLong float64
			var loc pkbLocation
			if lat, long, ok := parseLatLong(flagNear); ok {
				refLat, refLong = lat, long
				if flagCity == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--city is required when --near is coordinates"))
				}
				l, err := resolveCity(ctx, flags, flagCity)
				if err != nil {
					return err
				}
				loc = l
			} else {
				l, err := resolveCity(ctx, flags, flagNear)
				if err != nil {
					return err
				}
				loc = l
				refLat, refLong = l.Latitude, l.Longitude
				if flagCity != "" {
					cl, cerr := resolveCity(ctx, flags, flagCity)
					if cerr != nil {
						return cerr
					}
					loc = cl
				}
			}
			return nearestImpl(cmd, flags, ctx, args[0], loc, flagCategory, refLat, refLong)
		},
	}
	cmd.Flags().StringVar(&flagNear, "near", "", "Reference point: a city name or \"lat,long\" (required)")
	cmd.Flags().StringVar(&flagCity, "city", "", "City to scope branches to (required when --near is coordinates)")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Category id, only needed to resolve a merchant by name (1=Food)")
	return cmd
}

func parseLatLong(s string) (float64, float64, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	long, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if lat < -90 || lat > 90 || long < -180 || long > 180 {
		return 0, 0, false
	}
	return lat, long, true
}

func nearestImpl(cmd *cobra.Command, flags *rootFlags, ctx context.Context, merchant string, loc pkbLocation, category int, refLat, refLong float64) error {
	entityID, merchantName, err := resolveEntity(ctx, flags, merchant, loc, category, 5)
	if err != nil {
		return err
	}
	branches, apiName, err := fetchBranches(ctx, flags, fmt.Sprintf("%d", entityID), loc, 100)
	if err != nil {
		return err
	}
	if apiName != "" {
		merchantName = apiName
	}
	if len(branches) == 0 {
		if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
			return flags.printJSON(cmd, map[string]any{"merchant": merchantName, "merchant_id": entityID, "nearest": nil})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No branches found for %s in %s.\n", merchantName, loc.City)
		return nil
	}

	bestIdx := -1
	bestKm := math.MaxFloat64
	for i, b := range branches {
		// Skip branches with missing coordinates so a (0,0) branch can't be
		// picked as "nearest" with a bogus distance.
		if b.Latitude == 0 && b.Longitude == 0 {
			continue
		}
		km := haversineKm(refLat, refLong, b.Latitude, b.Longitude)
		if km < bestKm {
			bestKm = km
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
			return flags.printJSON(cmd, map[string]any{"merchant": merchantName, "merchant_id": entityID, "nearest": nil})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No branches with coordinates found for %s in %s.\n", merchantName, loc.City)
		return nil
	}
	best := branches[bestIdx]
	out := struct {
		Merchant      string    `json:"merchant"`
		MerchantID    int       `json:"merchant_id"`
		City          string    `json:"city"`
		Branch        pkbBranch `json:"branch"`
		DistanceKm    float64   `json:"distance_km"`
		DirectionsURL string    `json:"directions_url"`
	}{
		Merchant:      merchantName,
		MerchantID:    entityID,
		City:          loc.City,
		Branch:        best,
		DistanceKm:    math.Round(bestKm*100) / 100,
		DirectionsURL: mapsDirectionsURL(best.Latitude, best.Longitude),
	}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Nearest %s branch: %s (%.2f km)\n  %s\n  %s\n",
		merchantName, best.Name, out.DistanceKm, best.Address, out.DirectionsURL)
	return nil
}
