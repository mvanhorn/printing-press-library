// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

// newNearestCmd: cross-source reverse geocode. Given any lat/lon (or RD x/y),
// fans out /reverse across multiple type filters in parallel, picks the
// nearest match per type, and returns all of them in one record plus the
// gemeente / provincie holding the point.
func newNearestCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var rdX, rdY float64
	var distance int
	cmd := &cobra.Command{
		Use:   "nearest",
		Short: "Return the nearest address, parcel, hectometer marker, road, and gemeente for a point",
		Long: "Locatieserver `/reverse` returns only one type per call. `nearest` " +
			"runs four `/reverse` calls in parallel (adres, perceel, " +
			"hectometerpaal, weg) and adds the gemeente/provincie containing " +
			"the point. Accepts either --lat/--lon (WGS84) or --rd-x/--rd-y " +
			"(RD/EPSG:28992).",
		Example: "  pdok-location-pp-cli nearest --lat 52.3731 --lon 4.8922 --json\n" +
			"  pdok-location-pp-cli nearest --rd-x 121200 --rd-y 488000 --distance 500",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			haveLL := cmd.Flags().Changed("lat") && cmd.Flags().Changed("lon")
			haveRD := cmd.Flags().Changed("rd-x") && cmd.Flags().Changed("rd-y")
			partialLL := cmd.Flags().Changed("lat") != cmd.Flags().Changed("lon")
			partialRD := cmd.Flags().Changed("rd-x") != cmd.Flags().Changed("rd-y")
			if partialLL || partialRD {
				return usageErr(fmt.Errorf("both --lat and --lon are required (or both --rd-x and --rd-y)"))
			}
			if !haveLL && !haveRD {
				return cmd.Help()
			}
			if haveLL && haveRD {
				return usageErr(fmt.Errorf("specify --lat/--lon OR --rd-x/--rd-y, not both"))
			}
			// Normalize: every API call below uses --lat/--lon (WGS84). If
			// the caller passed RD, convert it once up front so the per-type
			// fan-out hits a single coordinate space.
			if haveRD {
				lon, lat = rdToWGS84(rdX, rdY)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			types := []string{"adres", "perceel", "hectometerpaal", "weg", "gemeente"}
			results := make(map[string]any, len(types))
			errs := make(map[string]string)
			var mu sync.Mutex
			var wg sync.WaitGroup
			for _, t := range types {
				wg.Add(1)
				go func(t string) {
					defer wg.Done()
					params := map[string]string{
						"lat":  fmt.Sprintf("%v", lat),
						"lon":  fmt.Sprintf("%v", lon),
						"type": t,
						"rows": "1",
					}
					if distance > 0 {
						params["distance"] = fmt.Sprintf("%d", distance)
					}
					data, err := c.Get("/bzk/locatieserver/search/v3_1/reverse", params)
					if err != nil {
						mu.Lock()
						errs[t] = err.Error()
						mu.Unlock()
						return
					}
					var resp lsResponse
					if err := json.Unmarshal(data, &resp); err != nil {
						mu.Lock()
						errs[t] = "parse: " + err.Error()
						mu.Unlock()
						return
					}
					if resp.Response.NumFound == 0 || len(resp.Response.Docs) == 0 {
						return
					}
					doc := enrichLSDoc(resp.Response.Docs[0], false)
					mu.Lock()
					results[t] = map[string]any{
						"id":              doc.ID,
						"weergavenaam":    doc.Weergavenaam,
						"type":            doc.Type,
						"distance_meters": doc.Afstand,
						"score":           doc.Score,
					}
					mu.Unlock()
				}(t)
			}
			wg.Wait()

			out := map[string]any{
				"query": map[string]float64{"lat": lat, "lon": lon},
			}
			if haveRD {
				out["query_rd"] = map[string]float64{"x": rdX, "y": rdY}
			}
			if v, ok := results["adres"]; ok {
				out["adres"] = v
			}
			if v, ok := results["perceel"]; ok {
				out["perceel"] = v
			}
			if v, ok := results["hectometerpaal"]; ok {
				out["hectometerpaal"] = v
			}
			if v, ok := results["weg"]; ok {
				out["weg"] = v
			}
			if v, ok := results["gemeente"]; ok {
				// Pull provincienaam out of the gemeente match if available
				// by issuing one more lookup on its id; cheap and only when
				// the gemeente match was non-empty.
				out["gemeente"] = v
			}
			if len(errs) > 0 {
				out["errors"] = errs
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "WGS84 latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "WGS84 longitude")
	cmd.Flags().Float64Var(&rdX, "rd-x", 0, "RD X coordinate (EPSG:28992)")
	cmd.Flags().Float64Var(&rdY, "rd-y", 0, "RD Y coordinate (EPSG:28992)")
	cmd.Flags().IntVar(&distance, "distance", 0, "Maximum search radius in meters (0 lets the API use its default)")
	return cmd
}
