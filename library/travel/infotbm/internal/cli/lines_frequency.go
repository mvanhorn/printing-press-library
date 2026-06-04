// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/client"
)

func newNovelLinesFrequencyCmd(flags *rootFlags) *cobra.Command {
	var flagLine string

	cmd := &cobra.Command{
		Use:   "frequency",
		Short: "Compute average headways per hour from the SIRI estimated timetable",
		Example: strings.Trim(`
  # Show frequency analysis for tram line A
  infotbm-pp-cli lines frequency --line A

  # Show frequency for bus line 1 in JSON
  infotbm-pp-cli lines frequency --line 1 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute average headways per hour for line from estimated timetable")
				return nil
			}
			if flagLine == "" {
				return usageErr(fmt.Errorf("--line is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			lineShort := strings.TrimSpace(flagLine)

			// Resolve short line name to full SIRI LineRef
			fullLineRef, err := resolveSIRILineRef(cmd.Context(), c, lineShort)
			if err != nil {
				return fmt.Errorf("resolving line %q: %w", lineShort, err)
			}

			// Fetch estimated timetable filtered by line and direction
			params := map[string]string{
				"LineRef":      fullLineRef,
				"DirectionRef": "0",
			}
			data, err := c.Get(cmd.Context(), "/siri/2.0/bordeaux/estimated-timetable.json", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Also fetch direction 1 and merge
			params1 := map[string]string{
				"LineRef":      fullLineRef,
				"DirectionRef": "1",
			}
			data1, err := c.Get(cmd.Context(), "/siri/2.0/bordeaux/estimated-timetable.json", params1)
			if err != nil {
				// Non-fatal: direction 1 may not exist
				data1 = nil
			}

			// Parse the timetable to extract departure times
			departures := extractDepartureTimes(data, fullLineRef)
			if data1 != nil {
				departures = append(departures, extractDepartureTimes(data1, fullLineRef)...)
			}
			if len(departures) == 0 {
				return fmt.Errorf("no departures found for line %q in estimated timetable", flagLine)
			}

			// Group departures by hour and compute headways
			type hourFrequency struct {
				Hour           int     `json:"hour"`
				HourLabel      string  `json:"hour_label"`
				DepartureCount int     `json:"departure_count"`
				AvgHeadwayMins float64 `json:"avg_headway_mins"`
				MinHeadwayMins float64 `json:"min_headway_mins,omitempty"`
				MaxHeadwayMins float64 `json:"max_headway_mins,omitempty"`
			}

			// Sort departures
			sort.Slice(departures, func(i, j int) bool {
				return departures[i].Before(departures[j])
			})

			// Group by hour
			hourBuckets := make(map[int][]time.Time)
			for _, d := range departures {
				h := d.Hour()
				hourBuckets[h] = append(hourBuckets[h], d)
			}

			results := make([]hourFrequency, 0)
			hours := make([]int, 0, len(hourBuckets))
			for h := range hourBuckets {
				hours = append(hours, h)
			}
			sort.Ints(hours)

			for _, h := range hours {
				deps := hourBuckets[h]
				sort.Slice(deps, func(i, j int) bool { return deps[i].Before(deps[j]) })

				hf := hourFrequency{
					Hour:           h,
					HourLabel:      fmt.Sprintf("%02d:00-%02d:59", h, h),
					DepartureCount: len(deps),
				}

				if len(deps) > 1 {
					var totalGap float64
					var minGap, maxGap float64
					var validGaps int
					minGap = 999
					for i := 1; i < len(deps); i++ {
						gap := deps[i].Sub(deps[i-1]).Minutes()
						if gap <= 0 {
							continue
						}
						validGaps++
						totalGap += gap
						if gap < minGap {
							minGap = gap
						}
						if gap > maxGap {
							maxGap = gap
						}
					}
					if validGaps > 0 && totalGap > 0 {
						hf.AvgHeadwayMins = totalGap / float64(validGaps)
						hf.MinHeadwayMins = minGap
						hf.MaxHeadwayMins = maxGap
					}
				} else {
					// Only one departure in this hour, headway is indeterminate
					hf.AvgHeadwayMins = 60 // effectively one per hour
				}

				results = append(results, hf)
			}

			view := struct {
				Line      string          `json:"line"`
				TotalDeps int             `json:"total_departures"`
				Frequency []hourFrequency `json:"frequency_by_hour"`
			}{
				Line:      flagLine,
				TotalDeps: len(departures),
				Frequency: results,
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagLine, "line", "", "Line short name (required, e.g. A, B, 1)")
	return cmd
}

// extractDepartureTimes parses SIRI estimated timetable response for departure times.
func extractDepartureTimes(data json.RawMessage, lineRef string) []time.Time {
	var departures []time.Time
	lineUpper := strings.ToUpper(lineRef)

	// SIRI estimated timetable has nested structure:
	// ServiceDelivery > EstimatedTimetableDelivery > EstimatedJourneyVersionFrame > EstimatedVehicleJourney
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return departures
	}

	// Navigate through possible envelope structures
	var journeys []map[string]any
	extractVehicleJourneys(data, &journeys)

	for _, j := range journeys {
		// Check line reference matches — LineRef may be a string or object {"value": "..."}
		lr := ""
		for _, key := range []string{"LineRef", "lineRef", "line_ref"} {
			if v, ok := j[key].(string); ok {
				lr = v
				break
			}
			// Handle SIRI object form: {"value": "bordeaux:Line:59:LOC"}
			if v, ok := j[key].(map[string]any); ok {
				if val, ok := v["value"].(string); ok {
					lr = val
					break
				}
			}
		}
		if lr != "" {
			lrUpper := strings.ToUpper(lr)
			parts := strings.Split(lr, ":")
			shortUpper := strings.ToUpper(parts[len(parts)-1])
			if lrUpper != lineUpper && shortUpper != lineUpper && !strings.EqualFold(lr, lineRef) {
				continue
			}
		}

		// Extract departure times from EstimatedCalls or RecordedCalls
		// EstimatedCalls may be a dict {"EstimatedCall": [...]} wrapping the actual list
		for _, callsKey := range []string{"EstimatedCalls", "estimatedCalls", "EstimatedCall", "RecordedCalls"} {
			if calls, ok := j[callsKey]; ok {
				// Handle {"EstimatedCall": [...]} wrapper
				if cm, ok := calls.(map[string]any); ok {
					if inner, ok := cm["EstimatedCall"]; ok {
						extractTimesFromCalls(inner, &departures)
						continue
					}
				}
				extractTimesFromCalls(calls, &departures)
			}
		}
	}

	return departures
}

func extractVehicleJourneys(data json.RawMessage, out *[]map[string]any) {
	// Try direct array of journeys
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		*out = append(*out, arr...)
		return
	}

	// Try nested envelope
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return
	}

	// Recurse into known container keys. Break after the first key that
	// yields results to avoid double-counting when sibling envelope keys
	// (e.g. "EstimatedJourneyVersionFrame" and "EstimatedVehicleJourney")
	// both appear at the same level and contain overlapping journey data.
	before := len(*out)
	for _, key := range []string{
		"Siri", "ServiceDelivery", "EstimatedTimetableDelivery",
		"EstimatedJourneyVersionFrame", "EstimatedVehicleJourney",
		"estimatedVehicleJourney", "vehicleJourneys",
		"data", "results",
	} {
		if inner, ok := obj[key]; ok {
			extractVehicleJourneys(inner, out)
			if len(*out) > before {
				break
			}
		}
	}

}

// resolveSIRILineRef maps a short line name (e.g. "A", "1") to the full SIRI
// LineRef format (e.g. "bordeaux:Line:59:LOC") by querying the lines-discovery endpoint.
func resolveSIRILineRef(ctx context.Context, c *client.Client, shortName string) (string, error) {
	// If it already looks like a full ref, return as-is
	if strings.Contains(shortName, ":") {
		return shortName, nil
	}

	data, err := c.Get(ctx, "/siri/2.0/bordeaux/lines-discovery.json", nil)
	if err != nil {
		return "", fmt.Errorf("fetching lines-discovery: %w", err)
	}

	// Parse the Siri > LinesDelivery > AnnotatedLineRef structure
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("parsing lines-discovery response: %w", err)
	}

	// Navigate through Siri wrapper if present
	if siri, ok := root["Siri"]; ok {
		if err := json.Unmarshal(siri, &root); err != nil {
			return "", fmt.Errorf("parsing Siri wrapper: %w", err)
		}
	}
	if ld, ok := root["LinesDelivery"]; ok {
		if err := json.Unmarshal(ld, &root); err != nil {
			return "", fmt.Errorf("parsing LinesDelivery: %w", err)
		}
	}

	// Get AnnotatedLineRef array
	var lines []map[string]any
	if alr, ok := root["AnnotatedLineRef"]; ok {
		if err := json.Unmarshal(alr, &lines); err != nil {
			return "", fmt.Errorf("parsing AnnotatedLineRef: %w", err)
		}
	}

	upperShort := strings.ToUpper(strings.TrimSpace(shortName))
	for _, line := range lines {
		// LineRef may be a string or object {"value": "..."}
		lineRefStr := ""
		if v, ok := line["LineRef"].(string); ok {
			lineRefStr = v
		} else if v, ok := line["LineRef"].(map[string]any); ok {
			if val, ok := v["value"].(string); ok {
				lineRefStr = val
			}
		}
		if lineRefStr == "" {
			continue
		}

		// Check LineCode first — it has the short code (e.g. "A", "1")
		// LineCode may be an object {"value": "1", "lang": "fr"}
		lineCode := ""
		if v, ok := line["LineCode"].(string); ok {
			lineCode = v
		} else if v, ok := line["LineCode"].(map[string]any); ok {
			if val, ok := v["value"].(string); ok {
				lineCode = val
			}
		}
		if lineCode != "" && strings.ToUpper(lineCode) == upperShort {
			return lineRefStr, nil
		}

		// Check LineName — may be a string, object {"value": "..."}, or
		// array [{"value": "Lianes 1", "lang": "fr"}]
		for _, name := range extractSIRITextValues(line, "LineName") {
			if strings.ToUpper(name) == upperShort {
				return lineRefStr, nil
			}
		}
	}

	return "", fmt.Errorf("line %q not found in SIRI lines-discovery; use full LineRef format like bordeaux:Line:59:LOC", shortName)
}

// extractSIRITextValues extracts text values from a SIRI field that may be
// a string, an object {"value": "..."}, or an array [{"value": "...", "lang": "..."}].
func extractSIRITextValues(item map[string]any, key string) []string {
	raw, ok := item[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return []string{v}
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			return []string{val}
		}
	case []any:
		var vals []string
		for _, elem := range v {
			if m, ok := elem.(map[string]any); ok {
				if val, ok := m["value"].(string); ok {
					vals = append(vals, val)
				}
			}
		}
		return vals
	}
	return nil
}

func extractTimesFromCalls(calls any, departures *[]time.Time) {
	var callSlice []map[string]any
	switch v := calls.(type) {
	case []any:
		for _, c := range v {
			if cm, ok := c.(map[string]any); ok {
				callSlice = append(callSlice, cm)
			}
		}
	case map[string]any:
		callSlice = []map[string]any{v}
	default:
		// Try JSON re-parse
		raw, err := json.Marshal(calls)
		if err != nil {
			return
		}
		json.Unmarshal(raw, &callSlice)
	}

	for _, call := range callSlice {
		for _, key := range []string{
			"ExpectedDepartureTime", "expectedDepartureTime",
			"AimedDepartureTime", "aimedDepartureTime",
		} {
			if ts, ok := call[key].(string); ok && ts != "" {
				if t, err := time.Parse(time.RFC3339, ts); err == nil {
					*departures = append(*departures, t)
					break
				}
				// Try other common formats
				for _, layout := range []string{
					"2006-01-02T15:04:05",
					"2006-01-02T15:04:05Z07:00",
					"2006-01-02 15:04:05",
				} {
					if t, err := time.Parse(layout, ts); err == nil {
						*departures = append(*departures, t)
						break
					}
				}
				break
			}
		}
	}
}
