// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

// rankSpots orders purchasable spots by price, then walk. Price leads because it is
// the difference people act on; distance breaks ties and caps via --max-walk.
func rankSpots(spots []consumer.SearchSpot, maxWalkM, maxCents int) []consumer.SearchSpot {
	var out []consumer.SearchSpot
	for _, s := range spots {
		if !s.Available || s.PriceCents <= 0 {
			continue
		}
		if maxWalkM > 0 && s.DistanceMeters > maxWalkM {
			continue
		}
		if maxCents > 0 && s.PriceCents > maxCents {
			continue
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PriceCents != out[j].PriceCents {
			return out[i].PriceCents < out[j].PriceCents
		}
		return out[i].DistanceMeters < out[j].DistanceMeters
	})
	return out
}

func newNovelBestCmd(flags *rootFlags) *cobra.Command {
	var address, airport, starts, ends, maxTotal string
	var maxWalk, alternatives int
	cmd := &cobra.Command{
		Use:   "best",
		Short: "Get one ranked parking pick plus alternatives for a place and time window.",
		Long: "Use this command to pick a garage near an address or airport for a new trip.\n" +
			"It ranks by cheapest total, then shortest distance, across up to three pages of results. Use --max-walk to limit distance.\n" +
			"Do NOT use this command to repeat a garage from a past reservation; use 'rebook' instead.",
		Example:     "  spothero-pp-cli best --address '233 S Wacker Dr, Chicago, IL' --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "best")
			}
			if starts == "" || ends == "" {
				return usageErr(fmt.Errorf("--starts and --ends are required"))
			}
			if (strings.TrimSpace(address) == "") == (strings.TrimSpace(airport) == "") {
				return usageErr(fmt.Errorf("provide exactly one of --address or --airport"))
			}
			capCents, err := parseDollars(maxTotal)
			if err != nil {
				return usageErr(err)
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			q := consumer.SearchQuery{Address: address, Starts: starts, Ends: ends}
			if airport != "" {
				lat, lng, err := c.AirportPoint(airport)
				if err != nil {
					return mapConsumerErr(err)
				}
				q.Address, q.Latitude, q.Longitude = "", lat, lng
			}
			res, err := c.SearchPages(q, 3)
			if err != nil {
				return mapConsumerErr(err)
			}
			ranked := rankSpots(res.Results, maxWalk, capCents)
			report := map[string]any{"ranking": "cheapest total first, then shortest distance", "more_results_not_considered": res.NextURL != "", "location_warning": res.LocationWarning, "resolved_city": res.Params.CitySlug, "considered": len(res.Results), "qualifying": len(ranked), "city_slug": res.Params.CitySlug, "timezone_note": res.TimezoneNote}
			if len(ranked) == 0 {
				report["pick"] = nil
				report["message"] = "Nothing available for this place and window within any limits you set."
				return flags.printJSON(cmd, report)
			}
			report["pick"] = ranked[0]
			if n := min(alternatives, len(ranked)-1); n > 0 {
				report["alternatives"] = ranked[1 : n+1]
			}
			report["next"] = fmt.Sprintf("spothero-pp-cli book preview --facility-id %d --starts %s --ends %s", ranked[0].FacilityID, starts, ends)
			return flags.printJSON(cmd, report)
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address or place")
	cmd.Flags().StringVar(&airport, "airport", "", "IATA airport code, for example ORD")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end (local or RFC3339)")
	cmd.Flags().StringVar(&maxTotal, "max-total", "", "Ignore spots above this many dollars")
	cmd.Flags().IntVar(&maxWalk, "max-walk", 0, "Ignore spots farther than this many meters")
	cmd.Flags().IntVar(&alternatives, "alternatives", 3, "How many runner-up spots to show")
	return cmd
}
