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

func newNovelStationsNearbyCmd(f *rootFlags) *cobra.Command {
	var o hcOptions
	var lat, lon, radius float64
	var purpose string
	var available bool
	c := &cobra.Command{Use: "nearby", Short: "Rank nearby pickup or return stations with explicit source states.", Example: "  hello-cycling-pp-cli stations nearby --lat 35.697315 --lon 139.704995 --purpose pickup --vehicle-type 2 --limit 5 --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}

	c.Flags().BoolVar(&o.offline, "offline", false, "Read only the explicitly synchronized local station snapshot")
	c.Flags().StringVar(&o.db, "snapshot-db", "", "SQLite snapshot path; defaults to the CLI data directory")
	c.Flags().DurationVar(&o.age, "status-max-age", 5*time.Minute, "Maximum source status age before counts are treated as stale")
	c.Flags().IntVar(&o.limit, "limit", 10, "Maximum result rows, between 1 and 50 (trip comparisons: 10)")
	c.Flags().StringVar(&o.vehicle, "vehicle-type", "", "Optional GBFS vehicle class ID; use vehicles rules to inspect current classes")

	c.Flags().Float64Var(&lat, "lat", 0, "Explicit search latitude, between -90 and 90 degrees")
	c.Flags().Float64Var(&lon, "lon", 0, "Explicit search longitude, between -180 and 180 degrees")
	c.Flags().Float64Var(&radius, "radius-m", 1500, "Straight-line discovery radius in meters, between 1 and 10000")
	c.Flags().StringVar(&purpose, "purpose", "pickup", "Which state to filter: pickup or return")
	c.Flags().BoolVar(&available, "available-only", false, "Include only fresh available observations; closed/empty/full/unknown excluded")
	_ = c.MarkFlagRequired("lat")
	_ = c.MarkFlagRequired("lon")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcValidate(o, 50); e != nil {
			return e
		}
		if !cycling.ValidCoordinates(lat, lon) {
			return usageErr(fmt.Errorf("--lat/--lon must be finite valid geographic coordinates"))
		}
		if math.IsNaN(radius) || math.IsInf(radius, 0) || radius < 1 || radius > 10000 {
			return usageErr(fmt.Errorf("--radius-m must be between 1 and 10000"))
		}
		if purpose != "pickup" && purpose != "return" {
			return usageErr(fmt.Errorf("--purpose must be pickup or return"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "stations nearby")
		}
		s, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		if e = hcKnownVehicle(s, o.vehicle); e != nil {
			return e
		}
		rows, total := cycling.Nearby(s.Stations(time.Now(), o.age, o.vehicle), lat, lon, radius, purpose, available, o.limit)
		out := hcResult(s, src, rows, total)
		out["query"] = map[string]any{"latitude": lat, "longitude": lon, "radius_meters": radius, "purpose": purpose, "available_only": available}
		return hcPrint(c, f, out)
	}
	return c
}
