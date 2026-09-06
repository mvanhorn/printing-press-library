// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:client-call

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlanRouteLegsCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, sectionIndex: -1}
	var modes, selected string
	var planning bool
	cmd := &cobra.Command{Use: "legs", Short: "Read saved travel estimates between consecutive stops; no route optimization", Args: cobra.NoArgs,
		Example:     "  wanderlog-pp-cli plan route legs --target-key naertjcoixqrgrfc --day 1 --modes driving,walking --agent",
		Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
			want, err := travelModes(modes)
			if err != nil {
				return usageErr(err)
			}
			selected = strings.ToLower(strings.TrimSpace(selected))
			if selected != "" {
				found := false
				for _, m := range want {
					if m == selected {
						found = true
					}
				}
				if !found {
					return usageErr(fmt.Errorf("--travel-mode must be one of --modes"))
				}
			}
			trip, key, err := readTravelPlan(cmd, flags, opts)
			if err != nil {
				return err
			}
			sec, err := resolveSection(trip, opts.day, opts.sectionIndex, opts.sectionID)
			if err != nil {
				return usageErr(err)
			}
			report := buildTravelLegs(trip, sec, want, selected, planning)
			report["target_key"] = key
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		}}
	addPlanTargetFlags(cmd, &opts)
	addPlanSectionFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	cmd.Flags().StringVar(&modes, "modes", "driving,walking", "Comma-separated saved estimate modes: driving,walking,bicycling,transit")
	cmd.Flags().StringVar(&selected, "travel-mode", "", "Explicit mode for schedule slack; absent means no selected mode")
	cmd.Flags().BoolVar(&planning, "with-planning", false, "Include saved opening hours, operational status and visit duration estimates")
	return cmd
}

func readTravelPlan(cmd *cobra.Command, flags *rootFlags, opts planEditOptions) (map[string]any, string, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := planLiveClient(flags)
	if err != nil {
		return nil, "", err
	}
	key, err := resolveEditablePlanKey(opts)
	if err != nil {
		return nil, "", usageErr(err)
	}
	return fetchPlan(ctx, c, key, opts.clientSchemaVersion)
}

func travelModes(raw string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range strings.Split(raw, ",") {
		m = strings.ToLower(strings.TrimSpace(m))
		switch m {
		case "driving", "walking", "bicycling", "transit":
		default:
			return nil, fmt.Errorf("unsupported travel mode %q; use driving,walking,bicycling,transit", m)
		}
		if !seen[m] {
			out = append(out, m)
			seen[m] = true
		}
	}
	return out, nil
}

func travelStopID(b map[string]any) string {
	return firstNonEmpty(stringField(mapField(b, "place"), "place_id"), stringField(mapField(b, "place"), "placeId"), stringField(b, "placeId"), stringField(b, "place_id"))
}

func buildTravelLegs(trip map[string]any, sec resolvedSection, modes []string, selected string, planning bool) map[string]any {
	blocks := sec.Blocks
	stops := []map[string]any{}
	originals := []map[string]any{}
	for index, raw := range blocks {
		b, ok := raw.(map[string]any)
		if !ok || (travelStopID(b) == "" && stringField(b, "type") != "place" && mapField(b, "place") == nil && mapField(b, "hotel") == nil) {
			continue
		}
		p := mapField(b, "place")
		stop := map[string]any{"block_id": b["id"], "block_index": index, "place_id": travelStopID(b), "name": outlineBlock(trip, b, index, sec.Report.Date).Name}
		if planning {
			stop["planning"] = travelPlanningFields(trip, p)
		}
		stops = append(stops, stop)
		originals = append(originals, b)
	}
	saved, _ := planResource(trip, "distancesBetweenPlaces").(map[string]any)
	legs := []map[string]any{}
	for i := 1; i < len(stops); i++ {
		from, to := originals[i-1], originals[i]
		estimates := []map[string]any{}
		var chosen map[string]any
		for _, mode := range modes {
			estimate := savedTravelEstimate(saved, travelStopID(from), travelStopID(to), mode, from, to)
			estimates = append(estimates, estimate)
			if mode == selected {
				chosen = estimate
			}
		}
		leg := map[string]any{"from_block_id": from["id"], "to_block_id": to["id"], "from_place_id": travelStopID(from), "to_place_id": travelStopID(to), "estimates": estimates}
		if selected != "" {
			leg["schedule"] = travelScheduleSlack(from, to, chosen)
		}
		legs = append(legs, leg)
	}
	modeSource := "unselected"
	if selected != "" {
		modeSource = "explicit"
	}
	return map[string]any{"command": "plan route legs", "section": sec.Report, "source": "saved_api_estimate", "freshness": "unknown", "selected_mode": selected, "mode_source": modeSource, "stops": stops, "legs": legs, "totals_by_mode": travelLegTotals(legs, modes)}
}

func savedTravelEstimate(saved map[string]any, from, to, mode string, stops ...map[string]any) map[string]any {
	result := map[string]any{"mode": mode, "available": false}
	if from == "" || to == "" {
		result["reason"] = "missing_place_id"
		return result
	}
	// Match recorded direction explicitly. Reverse routes need not have equal costs.
	for _, raw := range saved {
		e, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if stringField(e, "fromPlaceId") != from || stringField(e, "toPlaceId") != to || strings.ToLower(stringField(e, "travelMode")) != mode {
			continue
		}
		if len(stops) == 2 {
			if reason := savedTravelCoordinateMismatch(e, stops[0], stops[1]); reason != "" {
				result["reason"] = reason
				return result
			}
		}
		route := mapField(e, "route")
		distance := mapField(route, "distance")
		duration := mapField(route, "duration")
		meters, mok := travelNumber(distance["value"])
		seconds, sok := travelNumber(duration["value"])
		if mok {
			result["distance_meters"] = meters
		}
		if sok {
			result["duration_seconds"] = seconds
		}
		result["available"] = mok && sok
		if !mok || !sok {
			result["reason"] = "incomplete_saved_estimate"
		}
		return result
	}
	result["reason"] = "no_saved_estimate"
	return result
}

func travelNumber(v any) (float64, bool) {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case int:
		n = float64(x)
	case int64:
		n = float64(x)
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0
}

func travelScheduleSlack(from, to, estimate map[string]any) map[string]any {
	out := map[string]any{"status": "unknown"}
	if estimate == nil || estimate["available"] != true {
		out["reason"] = "missing_travel_estimate"
		return out
	}
	end, eok := parseClockToMinutes(stringField(from, "endTime"), "")
	start, sok := parseClockToMinutes(stringField(to, "startTime"), "")
	if !eok || !sok {
		out["reason"] = "missing_schedule_times"
		return out
	}
	if start < end {
		out["reason"] = "overlapping_or_overnight_schedule"
		return out
	}
	seconds, _ := travelNumber(estimate["duration_seconds"])
	slack := float64((start-end)*60) - seconds
	out["slack_seconds"] = slack
	out["status"] = "fits"
	if slack < 0 {
		out["status"] = "conflict"
	}
	return out
}

func travelPlanningFields(trip, place map[string]any) map[string]any {
	out := map[string]any{"freshness": "unknown"}
	for _, k := range []string{"business_status", "opening_hours"} {
		if v, ok := place[k]; ok {
			out[k] = v
		}
	}
	metadata, _ := planResource(trip, "placeMetadata").([]any)
	for _, raw := range metadata {
		m, ok := raw.(map[string]any)
		if !ok || stringField(m, "placeId") == "" || stringField(m, "placeId") != firstNonEmpty(stringField(place, "place_id"), stringField(place, "placeId")) {
			continue
		}
		for _, k := range []string{"minMinutesSpent", "maxMinutesSpent", "openingPeriods", "utcOffset", "businessStatus", "permanentlyClosed"} {
			if v, ok := m[k]; ok && v != nil {
				out[k] = v
			}
		}
		break
	}
	return out
}

// Only complete legs contribute to totals; never disguise a partial sum as a day total.
func travelLegTotals(legs []map[string]any, modes []string) []map[string]any {
	out := []map[string]any{}
	for _, mode := range modes {
		meters, seconds := 0.0, 0.0
		missing := 0
		for _, leg := range legs {
			found := false
			for _, estimate := range leg["estimates"].([]map[string]any) {
				if estimate["mode"] == mode && estimate["available"] == true {
					d, _ := travelNumber(estimate["distance_meters"])
					t, _ := travelNumber(estimate["duration_seconds"])
					meters += d
					seconds += t
					found = true
					break
				}
			}
			if !found {
				missing++
			}
		}
		out = append(out, map[string]any{"mode": mode, "available_distance_meters": meters, "available_duration_seconds": seconds, "leg_count": len(legs), "missing_leg_count": missing, "complete": missing == 0})
	}
	return out
}

// Saved route endpoints are part of the cache identity: the same place id may
// have been re-geocoded since this estimate was generated. Do not use stale
// durations to assert that a tight connection fits.
func savedTravelCoordinateMismatch(estimate, from, to map[string]any) string {
	for _, endpoint := range []struct {
		prefix string
		block  map[string]any
	}{{"from", from}, {"to", to}} {
		location := mapField(mapField(mapField(endpoint.block, "place"), "geometry"), "location")
		for _, axis := range []struct {
			suffix, key string
			bound       float64
		}{{"Lat", "lat", 90}, {"Lng", "lng", 180}} {
			cached, exists := estimate[endpoint.prefix+axis.suffix]
			if !exists {
				continue
			}
			a, aok := travelCoordinate(cached, axis.bound)
			b, bok := travelCoordinate(location[axis.key], axis.bound)
			if !aok {
				return "invalid_saved_coordinates"
			}
			if !bok {
				return "missing_current_coordinates"
			}
			if math.Abs(a-b) > 0.00001 {
				return "saved_coordinates_changed"
			}
		}
	}
	return ""
}

func travelCoordinate(value any, bound float64) (float64, bool) {
	var n float64
	switch v := value.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0) && math.Abs(n) <= bound
}

// fetchPlan deliberately retains raw resource values for the copy pipeline.
// Decode only the resource requested by these slim read views, rather than
// changing that established envelope contract or assuming fixture-only maps.
func planResource(trip map[string]any, name string) any {
	var value any
	switch resources := trip["_resources"].(type) {
	case map[string]json.RawMessage:
		value = resources[name]
	case map[string]any:
		value = resources[name]
	default:
		return nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		var decoded any
		if json.Unmarshal(raw, &decoded) != nil {
			return nil
		}
		return decoded
	}
	return value
}
