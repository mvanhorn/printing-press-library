// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Puts nearby-buoy observed swell next to the
// spot's forecast swell so you can sanity-check the model against reality.
// Buoy payload shapes vary, so observed readings are parsed defensively and the
// raw buoy objects are preserved in JSON output.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type buoyReading struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Status    string          `json:"status,omitempty"`
	Distance  float64         `json:"distance_km,omitempty"`
	Lat       float64         `json:"-"`
	Lon       float64         `json:"-"`
	SwellHt   float64         `json:"observed_swell_ft,omitempty"`
	SwellPer  float64         `json:"observed_period_s,omitempty"`
	SwellDir  float64         `json:"observed_direction_deg,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	parsedAny bool
}

type buoyCheckView struct {
	Spot            string        `json:"spot"`
	SpotID          string        `json:"spot_id"`
	ForecastSwellFt float64       `json:"forecast_swell_ft"`
	ForecastPeriodS float64       `json:"forecast_period_s"`
	ForecastDirDeg  float64       `json:"forecast_direction_deg"`
	Buoys           []buoyReading `json:"buoys"`
	Note            string        `json:"note,omitempty"`
}

// firstNumber pulls the first present numeric value among candidate keys.
func firstNumber(obj map[string]json.RawMessage, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := cliutil.ExtractNumber(obj, k); ok {
			return v
		}
	}
	return 0
}

func firstString(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		if raw, ok := obj[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// parseBuoys extracts buoy readings from the shapes Surfline may return. The
// live /kbyg/buoys/nearby shape is {associated, data:[{sourceId, name,
// latitude, longitude, status, latestData:{height, period, direction}}]}; older
// wrapper shapes use {buoys:[...]} or a bare array. Observations may sit at the
// top level or under latestData/latest/observation.
func parseBuoys(data json.RawMessage) []buoyReading {
	var probe struct {
		Data  json.RawMessage `json:"data"`
		Buoys json.RawMessage `json:"buoys"`
	}
	_ = json.Unmarshal(data, &probe)
	var items []json.RawMessage
	tryArray := func(raw json.RawMessage) bool {
		if len(raw) == 0 {
			return false
		}
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
			items = arr
			return true
		}
		// data may itself be an object wrapping buoys.
		var obj struct {
			Buoys []json.RawMessage `json:"buoys"`
		}
		if json.Unmarshal(raw, &obj) == nil && len(obj.Buoys) > 0 {
			items = obj.Buoys
			return true
		}
		return false
	}
	switch {
	case tryArray(probe.Data):
	case tryArray(probe.Buoys):
	default:
		_ = json.Unmarshal(data, &items)
	}

	out := make([]buoyReading, 0, len(items))
	for _, it := range items {
		var obj map[string]json.RawMessage
		if json.Unmarshal(it, &obj) != nil {
			continue
		}
		readSrc := map[string]json.RawMessage{}
		for k, v := range obj {
			readSrc[k] = v
		}
		for _, nestKey := range []string{"latestData", "latest", "observation", "waves"} {
			if raw, ok := obj[nestKey]; ok {
				var inner map[string]json.RawMessage
				if json.Unmarshal(raw, &inner) == nil {
					for k, v := range inner {
						readSrc[k] = v
					}
				}
			}
		}
		r := buoyReading{
			ID:       firstString(obj, "sourceId", "stationId", "_id", "id"),
			Name:     firstString(obj, "name", "title", "label"),
			Status:   firstString(obj, "status"),
			Lat:      firstNumber(obj, "latitude", "lat"),
			Lon:      firstNumber(obj, "longitude", "lon"),
			Distance: firstNumber(obj, "distance", "distanceAway", "distanceKm"),
			SwellHt:  firstNumber(readSrc, "height", "significantWaveHeight", "waveHeight", "swellHeight"),
			SwellPer: firstNumber(readSrc, "period", "dominantWavePeriod", "peakPeriod", "swellPeriod"),
			SwellDir: firstNumber(readSrc, "direction", "meanWaveDirection", "waveDirection", "swellDirection"),
			Raw:      it,
		}
		r.parsedAny = r.SwellHt != 0 || r.SwellPer != 0 || r.SwellDir != 0
		out = append(out, r)
	}
	return out
}

// isBuoyStale reports whether a buoy's status indicates it is not actively
// reporting, so its observed swell should not be trusted as a live reading.
// Surfline marks dormant stations OFFLINE/INACTIVE; an empty status is treated
// as unknown (not stale) to avoid over-warning when the field is absent.
func isBuoyStale(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "OFFLINE", "INACTIVE", "DECOMMISSIONED", "RETIRED", "OUT_OF_SERVICE":
		return true
	default:
		return false
	}
}

// haversineKm returns the great-circle distance between two lat/lon points in km.
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func fetchBuoysRaw(ctx context.Context, c *client.Client, lat, lon float64, limit int) (json.RawMessage, error) {
	params := map[string]string{
		"latitude":  fmt.Sprintf("%.5f", lat),
		"longitude": fmt.Sprintf("%.5f", lon),
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	return c.Get(ctx, "/kbyg/buoys/nearby", params)
}

func newNovelBuoyCheckCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "buoy-check <spotId>",
		Short: "Show nearby-buoy observed swell against the spot's wave forecast for the same window, side by side.",
		Long: "Joins nearby-buoy observations with the spot's wave forecast so you can sanity-check the model against reality.\n\n" +
			"Use this command to sanity-check the forecast against live buoy observations. " +
			"It does not judge spots; use 'rank' to choose between them.",
		Example: strings.Trim(`
  surfline-pp-cli buoy-check 5842041f4e65fad6a7708807
  surfline-pp-cli buoy-check 5842041f4e65fad6a7708807 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch spot location, nearby buoys, and the spot's wave forecast")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required"))
			}
			spotID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			name := fetchSpotName(ctx, c, spotID)
			lat, lon, ok := fetchSpotLatLon(ctx, c, spotID)
			if !ok {
				return notFoundErr(fmt.Errorf("could not resolve coordinates for spot %q", spotID))
			}
			waves, err := fetchWave(ctx, c, spotID, 1, 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := buoyCheckView{Spot: name, SpotID: spotID, Buoys: []buoyReading{}}
			if len(waves) > 0 {
				if sw, ok := waves[0].topSwell(); ok {
					view.ForecastSwellFt = sw.Height
					view.ForecastPeriodS = sw.Period
					view.ForecastDirDeg = sw.Direction
				}
			}

			buoyData, err := fetchBuoysRaw(ctx, c, lat, lon, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view.Buoys = parseBuoys(buoyData)
			// Compute distance from the spot when the buoy carries coordinates
			// and the API did not already provide one, then sort nearest-first.
			for i := range view.Buoys {
				b := &view.Buoys[i]
				if b.Distance == 0 && (b.Lat != 0 || b.Lon != 0) {
					b.Distance = haversineKm(lat, lon, b.Lat, b.Lon)
				}
			}
			sort.SliceStable(view.Buoys, func(i, j int) bool { return view.Buoys[i].Distance < view.Buoys[j].Distance })
			parsedCount := 0
			for _, b := range view.Buoys {
				if b.parsedAny {
					parsedCount++
				}
			}
			offlineCount := 0
			for _, b := range view.Buoys {
				if isBuoyStale(b.Status) {
					offlineCount++
				}
			}
			if len(view.Buoys) == 0 {
				view.Note = "no nearby buoys returned for this spot's location"
			} else if parsedCount == 0 {
				view.Note = "buoys found but observed-swell fields could not be parsed; see raw objects in --json output"
			} else if offlineCount == len(view.Buoys) {
				view.Note = "warning: all nearby buoys report a non-active status; observed swell may be stale — do not treat it as a live reading"
			} else if offlineCount > 0 {
				view.Note = fmt.Sprintf("warning: %d of %d nearby buoys report a non-active status; their observed swell may be stale", offlineCount, len(view.Buoys))
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", bold(name), spotID)
			fmt.Fprintf(out, "Forecast swell: %.1fft @ %.0fs  %.0f°\n", view.ForecastSwellFt, view.ForecastPeriodS, view.ForecastDirDeg)
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
				// Only the empty / unparseable notes have no table to show.
				if len(view.Buoys) == 0 || parsedCount == 0 {
					return nil
				}
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "BUOY\tSTATUS\tDIST\tOBS_SWELL\tOBS_PERIOD\tOBS_DIR")
			for _, b := range view.Buoys {
				label := b.Name
				if label == "" {
					label = b.ID
				}
				status := b.Status
				if status == "" {
					status = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%.1fft\t%.0fs\t%.0f°\n", truncate(label, 24), status, b.Distance, b.SwellHt, b.SwellPer, b.SwellDir)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Max nearby buoys to return")
	return cmd
}
