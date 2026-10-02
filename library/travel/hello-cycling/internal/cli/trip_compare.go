// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"math"
	"time"
)

func newNovelTripCompareCmd(f *rootFlags) *cobra.Command {
	var o hcOptions
	var fl, fn, tl, tn, radius float64
	c := &cobra.Command{Use: "compare", Short: "Compare compatible pickup/dropoff choices from two explicit coordinates.", Example: "  hello-cycling-pp-cli trip compare --from-lat 35.697315 --from-lon 139.704995 --to-lat 35.707252 --to-lon 139.777587 --vehicle-type 2 --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}

	c.Flags().BoolVar(&o.offline, "offline", false, "Read only the explicitly synchronized local station snapshot")
	c.Flags().StringVar(&o.db, "snapshot-db", "", "SQLite snapshot path; defaults to the CLI data directory")
	c.Flags().DurationVar(&o.age, "status-max-age", 5*time.Minute, "Maximum source status age before counts are treated as stale")
	c.Flags().IntVar(&o.limit, "limit", 5, "Maximum result rows, between 1 and 50 (trip comparisons: 10)")
	c.Flags().StringVar(&o.vehicle, "vehicle-type", "", "Optional GBFS vehicle class ID; use vehicles rules to inspect current classes")

	c.Flags().Float64Var(&fl, "from-lat", 0, "Explicit origin latitude in geographic degrees")
	c.Flags().Float64Var(&fn, "from-lon", 0, "Explicit origin longitude in geographic degrees")
	c.Flags().Float64Var(&tl, "to-lat", 0, "Explicit destination latitude in geographic degrees")
	c.Flags().Float64Var(&tn, "to-lon", 0, "Explicit destination longitude in geographic degrees")
	c.Flags().Float64Var(&radius, "radius-m", 1500, "Maximum straight-line distance from each endpoint in meters, up to 10000")
	for _, k := range []string{"from-lat", "from-lon", "to-lat", "to-lon", "vehicle-type"} {
		_ = c.MarkFlagRequired(k)
	}
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcValidate(o, 10); e != nil {
			return e
		}
		if o.vehicle == "" {
			return usageErr(fmt.Errorf("--vehicle-type is required for compatible trip comparison"))
		}
		if !cycling.ValidCoordinates(fl, fn) || !cycling.ValidCoordinates(tl, tn) {
			return usageErr(fmt.Errorf("origin and destination --lat/--lon flags must be finite valid coordinates"))
		}
		if math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 1 || radius > 10000 {
			return usageErr(fmt.Errorf("--radius-m must be between 1 and 10000"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "trip compare")
		}
		s, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		if e = hcKnownVehicle(s, o.vehicle); e != nil {
			return e
		}
		stations := s.Stations(time.Now(), o.age, o.vehicle)
		pairs := cycling.Compare(stations, fl, fn, tl, tn, radius, o.limit)
		out := hcResult(s, src, pairs, len(pairs))
		out["coverage"] = map[string]any{"vehicle_type": o.vehicle, "candidate_limit_per_endpoint": o.limit + 1, "radius_meters": radius, "availability": "fresh observed compatible generic class; confirm the actual vehicle in app"}
		if len(pairs) == 0 {
			out["no_pair_reason"] = "No fresh pickup and compatible return pair was observed within the candidate/radius bounds; stale, closed, empty, full and source-missing stations are excluded."
		}
		return hcPrint(c, f, out)
	}
	return c
}
