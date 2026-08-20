// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var (
		minMag      float64
		maxMag      float64
		since       string
		near        string
		radiusKm    float64
		bbox        string
		limit       int
		alertLevel  string
		eventType   string
		tsunamiOnly bool
		minFelt     int
	)
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "List recent earthquakes (default last 24h, M2.5+)",
		Long: `List recent earthquakes from the USGS FDSN Event service.

Friendly wrapper over 'events search' with sensible defaults:
- --since 24h  (default)
- --min-magnitude 2.5 (default)

Use --near "lat,lon" for a circle filter, or --bbox W,S,E,N for a rectangle.`,
		Example: strings.Trim(`
  # Default: last 24h, M2.5+
  usgs-earthquakes-pp-cli recent --json

  # M4.5+ in the last week worldwide
  usgs-earthquakes-pp-cli recent --since 7d --min-magnitude 4.5 --json

  # Within 500 km of San Francisco
  usgs-earthquakes-pp-cli recent --near 37.77,-122.42 --radius-km 500 --json

  # California bounding box, past 7 days
  usgs-earthquakes-pp-cli recent --bbox -125,32,-114,42 --since 7d --json

  # Newsworthy only
  usgs-earthquakes-pp-cli recent --since 1h --alert orange --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			startT, err := parseSinceArg(since)
			if err != nil {
				return usageErr(err)
			}

			params := map[string]string{
				"format":    "geojson",
				"starttime": fdsnTimeFormat(startT),
				"limit":     strconv.Itoa(limit),
				"orderby":   "time",
			}
			if minMag > 0 {
				params["minmagnitude"] = strconv.FormatFloat(minMag, 'f', -1, 64)
			}
			if maxMag > 0 {
				params["maxmagnitude"] = strconv.FormatFloat(maxMag, 'f', -1, 64)
			}
			if alertLevel != "" {
				params["alertlevel"] = alertLevel
			}
			if eventType != "" {
				params["eventtype"] = eventType
			}
			if minFelt > 0 {
				params["minfelt"] = strconv.Itoa(minFelt)
			}
			if near != "" {
				lat, lon, perr := parseLatLonPair(near)
				if perr != nil {
					return usageErr(perr)
				}
				params["latitude"] = strconv.FormatFloat(lat, 'f', -1, 64)
				params["longitude"] = strconv.FormatFloat(lon, 'f', -1, 64)
				if radiusKm <= 0 {
					radiusKm = 200
				}
				params["maxradiuskm"] = strconv.FormatFloat(radiusKm, 'f', -1, 64)
			}
			if bbox != "" {
				w, s, e, n, perr := parseBBox(bbox)
				if perr != nil {
					return usageErr(perr)
				}
				params["minlongitude"] = strconv.FormatFloat(w, 'f', -1, 64)
				params["minlatitude"] = strconv.FormatFloat(s, 'f', -1, 64)
				params["maxlongitude"] = strconv.FormatFloat(e, 'f', -1, 64)
				params["maxlatitude"] = strconv.FormatFloat(n, 'f', -1, 64)
			}

			data, err := c.Get("/query", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if tsunamiOnly {
				data = filterTsunamiOnly(data)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().Float64Var(&minMag, "min-magnitude", 2.5, "Minimum magnitude")
	cmd.Flags().Float64Var(&maxMag, "max-magnitude", 0, "Maximum magnitude (0 = no cap)")
	cmd.Flags().StringVar(&since, "since", "24h", "Lookback window (24h, 7d, 30m, or ISO 8601 timestamp)")
	cmd.Flags().StringVar(&near, "near", "", `Center point for circle filter, "lat,lon" (e.g. "37.77,-122.42")`)
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 0, "Circle filter radius in km (default 200 when --near is set)")
	cmd.Flags().StringVar(&bbox, "bbox", "", `Bounding box "W,S,E,N" (e.g. "-125,32,-114,42")`)
	cmd.Flags().IntVar(&limit, "limit", 100, "Max events to return (FDSN cap: 20000)")
	cmd.Flags().StringVar(&alertLevel, "alert", "", "PAGER alert level (green|yellow|orange|red)")
	cmd.Flags().StringVar(&eventType, "event-type", "", "Event type filter (e.g. earthquake, explosion, quarry)")
	cmd.Flags().BoolVar(&tsunamiOnly, "tsunami-only", false, "Filter to events that triggered tsunami warnings")
	cmd.Flags().IntVar(&minFelt, "min-felt", 0, "Minimum DYFI 'felt' reports")
	return cmd
}

// parseLatLonPair handles "lat,lon" or "lat, lon" forms.
func parseLatLonPair(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("--near expects \"lat,lon\" (got %q)", s)
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("--near values must be numeric (got %q)", s)
	}
	if lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("latitude out of range: %v", lat)
	}
	if lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("longitude out of range: %v", lon)
	}
	return lat, lon, nil
}

// parseBBox handles "W,S,E,N" form.
func parseBBox(s string) (w, sLat, e, n float64, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		err = fmt.Errorf("--bbox expects \"W,S,E,N\" (got %q)", s)
		return
	}
	var perr [4]error
	w, perr[0] = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	sLat, perr[1] = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	e, perr[2] = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	n, perr[3] = strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	for _, pe := range perr {
		if pe != nil {
			err = fmt.Errorf("--bbox values must be numeric (got %q)", s)
			return
		}
	}
	return
}

// filterTsunamiOnly walks a FeatureCollection and drops features whose
// properties.tsunami is not truthy.
func filterTsunamiOnly(data json.RawMessage) json.RawMessage {
	var fc struct {
		Type     string            `json:"type"`
		Metadata json.RawMessage   `json:"metadata"`
		Features []json.RawMessage `json:"features"`
		Bbox     json.RawMessage   `json:"bbox"`
	}
	if json.Unmarshal(data, &fc) != nil {
		return data
	}
	kept := make([]json.RawMessage, 0, len(fc.Features))
	for _, raw := range fc.Features {
		var feat struct {
			Properties struct {
				Tsunami int `json:"tsunami"`
			} `json:"properties"`
		}
		if json.Unmarshal(raw, &feat) == nil && feat.Properties.Tsunami != 0 {
			kept = append(kept, raw)
		}
	}
	fc.Features = kept
	out, err := json.Marshal(fc)
	if err != nil {
		return data
	}
	return out
}
