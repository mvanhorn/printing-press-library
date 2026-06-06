// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `route` — find Atlas Obscura wonders along the driving corridor between two
// cities (hand-authored flagship). Geocodes both endpoints, samples points along
// the path, geo-searches each, dedupes, scores, and returns corridor stops.
package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/cliutil"
)

func newNovelRouteCmd(flags *rootFlags) *cobra.Command {
	var minScore int
	var limit int
	var width float64
	var samples int

	cmd := &cobra.Command{
		Use:   "route <cityA> <cityB>",
		Short: "Find Atlas Obscura wonders along the driving corridor between two cities, not just in one place.",
		Long: "Find wonders along the corridor between two places. Both endpoints are geocoded\n" +
			"(Open-Meteo), the straight-line path is sampled, and each sample point is geo-searched.\n" +
			"Stops within --width miles of the path are deduped, scored, and returned best-first.\n" +
			"The corridor is a straight-line approximation, not turn-by-turn routing.\n" +
			"Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli route \"San Francisco\" \"Los Angeles\" --min-score 6 --limit 15 --json\n" +
			"  atlas-obscura-pp-cli route \"Denver\" \"Moab\" --width 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan the corridor between two places")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two places are required: route <cityA> <cityB>"))
			}
			if limit < 1 {
				limit = 15
			}
			if width <= 0 {
				width = 20
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			latA, lngA, labelA, err := resolvePoint(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			latB, lngB, labelB, err := resolvePoint(cmd.Context(), args[1])
			if err != nil {
				return err
			}

			stops, err := collectRoute(cmd.Context(), c, routeReq{
				latA: latA, lngA: lngA, latB: latB, lngB: lngB,
				width: width, samples: samples, minScore: minScore, limit: limit,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if s, derr := aoDB(cmd.Context()); derr == nil {
				for _, p := range stops {
					cachePlace(s, p)
				}
				_ = s.Close()
			}

			return aoEmitPlaces(cmd, flags, map[string]any{
				"from":           labelA,
				"to":             labelB,
				"corridor_miles": haversineMiles(latA, lngA, latB, lngB),
				"width_miles":    width,
			}, stops)
		},
	}
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Only include stops with at least this interestingness score (0-10 heuristic)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum number of corridor stops to return")
	cmd.Flags().Float64Var(&width, "width", 20, "Corridor half-width in miles (max distance off the straight-line path)")
	cmd.Flags().IntVar(&samples, "samples", 0, "Number of points to sample along the path (0 = auto from distance)")
	return cmd
}

type routeReq struct {
	latA, lngA, latB, lngB float64
	width                  float64
	samples                int
	minScore               int
	limit                  int
}

func collectRoute(ctx context.Context, c *client.Client, r routeReq) ([]AOPlace, error) {
	samples := r.samples
	if samples <= 0 {
		dist := haversineMiles(r.latA, r.lngA, r.latB, r.lngB)
		samples = int(dist/40) + 2
	}
	if samples < 2 {
		samples = 2
	}
	if samples > 12 {
		samples = 12
	}
	if cliutil.IsDogfoodEnv() && samples > 3 {
		samples = 3
	}

	// Track each stop's position along the corridor (t in [0,1], A→B) so we can
	// spread the limited result set across the whole route instead of clustering
	// near whichever endpoint happens to sort first. Scores frequently tie (the
	// interestingness heuristic is coarse), so without a corridor-aware tiebreaker
	// a plain score-then-title sort yields an alphabetical slice that drops every
	// mid-corridor wonder once --limit truncates.
	seen := map[int]bool{}
	corridorT := map[int]float64{}
	var stops []AOPlace
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(samples-1)
		lat := r.latA + (r.latB-r.latA)*t
		lng := r.lngA + (r.lngB-r.lngA)*t
		resp, err := aoNear(ctx, c, lat, lng, 1)
		if err != nil {
			return nil, err
		}
		for _, e := range resp.Results {
			p := e.toPlace()
			d, ok := parseDistanceMiles(p.DistanceFromQuery)
			if !ok && (p.Lat != 0 || p.Lng != 0) {
				d, ok = haversineMiles(lat, lng, p.Lat, p.Lng), true
			}
			if !ok {
				continue // can't verify within the corridor width
			}
			if d > r.width {
				break // distance-sorted; rest are farther
			}
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			corridorT[p.ID] = t
			p.Score = aoScore(p)
			stops = append(stops, p)
		}
	}

	// Drop below-threshold stops before selecting, so --min-score and --limit
	// compose correctly.
	if r.minScore > 0 {
		filtered := stops[:0:0]
		for _, p := range stops {
			if p.Score >= r.minScore {
				filtered = append(filtered, p)
			}
		}
		stops = filtered
	}

	// Best-first by score, then by corridor position (travel order A→B) so ties
	// spread along the route; title only as a final deterministic fallback.
	sort.SliceStable(stops, func(i, j int) bool {
		if stops[i].Score != stops[j].Score {
			return stops[i].Score > stops[j].Score
		}
		ti, tj := corridorT[stops[i].ID], corridorT[stops[j].ID]
		if ti != tj {
			return ti < tj
		}
		return stops[i].Title < stops[j].Title
	})

	if r.limit > 0 && len(stops) > r.limit {
		stops = corridorSpread(stops, corridorT, r.limit)
	}

	// Present the returned stops in travel order (A→B) so the itinerary reads
	// like a drive plan rather than a score ranking.
	sort.SliceStable(stops, func(i, j int) bool {
		ti, tj := corridorT[stops[i].ID], corridorT[stops[j].ID]
		if ti != tj {
			return ti < tj
		}
		if stops[i].Score != stops[j].Score {
			return stops[i].Score > stops[j].Score
		}
		return stops[i].Title < stops[j].Title
	})
	return stops, nil
}

// corridorSpread picks `limit` stops that both rank well and span the whole
// corridor. It walks the score-ranked list and greedily skips a candidate only
// when an already-picked stop sits very close to it on the path, until the slate
// fills — so the result is the high-scoring stops, de-clustered across A→B,
// rather than a dense knot near one endpoint. Falls back to plain rank order if
// spreading can't fill the slate.
func corridorSpread(ranked []AOPlace, corridorT map[int]float64, limit int) []AOPlace {
	if limit <= 0 || len(ranked) <= limit {
		return ranked
	}
	// Minimum corridor separation that scales with how many stops we want:
	// limit stops across [0,1] want roughly 1/limit spacing; relax to half that
	// so we don't starve the slate.
	minGap := 0.5 / float64(limit)
	picked := make([]AOPlace, 0, limit)
	pickedT := make([]float64, 0, limit)
	tooClose := func(t float64) bool {
		for _, pt := range pickedT {
			if diff := t - pt; diff < minGap && diff > -minGap {
				return true
			}
		}
		return false
	}
	for _, p := range ranked {
		if len(picked) >= limit {
			break
		}
		t := corridorT[p.ID]
		if tooClose(t) {
			continue
		}
		picked = append(picked, p)
		pickedT = append(pickedT, t)
	}
	// Backfill from rank order if de-clustering left the slate short.
	if len(picked) < limit {
		in := map[int]bool{}
		for _, p := range picked {
			in[p.ID] = true
		}
		for _, p := range ranked {
			if len(picked) >= limit {
				break
			}
			if !in[p.ID] {
				picked = append(picked, p)
			}
		}
	}
	return picked
}
