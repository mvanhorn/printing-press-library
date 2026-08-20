// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNearCmd(flags *rootFlags) *cobra.Command {
	var (
		radiusKm   float64
		since      string
		minMag     float64
		limit      int
		ordered    string
		latFlagVal float64
		lonFlagVal float64
	)
	cmd := &cobra.Command{
		Use:   "near [lat,lon]",
		Short: "Show recent earthquakes near a coordinate, ranked by distance + significance",
		Long: `Show recent earthquakes near a coordinate.

Coordinate can be passed as a single positional "lat,lon" argument, or via
--latitude/--longitude flags (useful for negative longitudes that Cobra would
otherwise parse as flags).`,
		Example: strings.Trim(`
  # Quakes within 500 km of San Francisco, past 7 days
  usgs-earthquakes-pp-cli near "37.77,-122.42" --radius-km 500 --since 7d --json

  # Same, using explicit flags
  usgs-earthquakes-pp-cli near --latitude 37.77 --longitude -122.42 --radius-km 500 --json

  # Within 50 km of Mount Hood, ordered by distance
  usgs-earthquakes-pp-cli near "45.3736,-121.6960" --radius-km 50 --since 30d --order distance --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				lat, lon float64
				latFlag  = cmd.Flag("latitude")
				lonFlag  = cmd.Flag("longitude")
				latSet   = latFlag != nil && latFlag.Changed
				lonSet   = lonFlag != nil && lonFlag.Changed
			)
			switch {
			case latSet && lonSet:
				lat, lon = latFlagVal, lonFlagVal
			case len(args) == 1:
				var err error
				lat, lon, err = parseLatLonPair(args[0])
				if err != nil {
					return usageErr(err)
				}
			default:
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if lat < -90 || lat > 90 {
				return usageErr(fmt.Errorf("latitude out of range: %v", lat))
			}
			if lon < -180 || lon > 180 {
				return usageErr(fmt.Errorf("longitude out of range: %v", lon))
			}
			startT, err := parseSinceArg(since)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"format":      "geojson",
				"starttime":   fdsnTimeFormat(startT),
				"latitude":    strconv.FormatFloat(lat, 'f', -1, 64),
				"longitude":   strconv.FormatFloat(lon, 'f', -1, 64),
				"maxradiuskm": strconv.FormatFloat(radiusKm, 'f', -1, 64),
				"limit":       strconv.Itoa(limit),
			}
			if minMag > 0 {
				params["minmagnitude"] = strconv.FormatFloat(minMag, 'f', -1, 64)
			}
			data, err := c.Get("/query", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Augment features with computed distance + sort.
			data = augmentWithDistance(data, lat, lon, ordered)

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().Float64Var(&latFlagVal, "latitude", 0, "Center latitude (alternative to positional lat,lon)")
	cmd.Flags().Float64Var(&lonFlagVal, "longitude", 0, "Center longitude (alternative to positional lat,lon)")
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 200, "Search radius in km")
	cmd.Flags().StringVar(&since, "since", "7d", "Lookback window (24h, 7d, 30m, or ISO 8601 timestamp)")
	cmd.Flags().Float64Var(&minMag, "min-magnitude", 0, "Minimum magnitude")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max events to return")
	cmd.Flags().StringVar(&ordered, "order", "time", "Sort order: time, distance, magnitude")
	return cmd
}

func augmentWithDistance(data json.RawMessage, lat, lon float64, order string) json.RawMessage {
	var fc map[string]json.RawMessage
	if json.Unmarshal(data, &fc) != nil {
		return data
	}
	rawFeatures, ok := fc["features"]
	if !ok {
		return data
	}
	var features []map[string]any
	if json.Unmarshal(rawFeatures, &features) != nil {
		return data
	}
	for _, f := range features {
		geom, _ := f["geometry"].(map[string]any)
		if geom == nil {
			continue
		}
		coords, _ := geom["coordinates"].([]any)
		if len(coords) < 2 {
			continue
		}
		eLon, ok1 := coords[0].(float64)
		eLat, ok2 := coords[1].(float64)
		if !ok1 || !ok2 {
			continue
		}
		dist := haversineKm(lat, lon, eLat, eLon)
		props, _ := f["properties"].(map[string]any)
		if props == nil {
			props = make(map[string]any)
			f["properties"] = props
		}
		props["distance_km"] = round2(dist)
	}
	// Sort.
	switch order {
	case "distance":
		sortByDistance(features)
	case "magnitude":
		sortByMagnitude(features)
	default:
		// FDSN already returns ordered by time desc; keep that.
	}
	reMarshaled, err := json.Marshal(features)
	if err != nil {
		return data
	}
	fc["features"] = reMarshaled
	out, err := json.Marshal(fc)
	if err != nil {
		return data
	}
	return out
}

func sortByDistance(features []map[string]any) {
	sort.Slice(features, func(i, j int) bool {
		return featureField(features[i], "distance_km") < featureField(features[j], "distance_km")
	})
}

func sortByMagnitude(features []map[string]any) {
	sort.Slice(features, func(i, j int) bool {
		return featureField(features[i], "mag") > featureField(features[j], "mag")
	})
}

func featureField(f map[string]any, key string) float64 {
	props, _ := f["properties"].(map[string]any)
	if props == nil {
		return 0
	}
	if v, ok := props[key].(float64); ok {
		return v
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
