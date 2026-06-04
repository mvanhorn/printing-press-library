// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/client"
)

type lineStopView struct {
	Order    int    `json:"order"`
	StopID   string `json:"stop_id"`
	StopName string `json:"stop_name"`
}

func newNovelLinesStopsCmd(flags *rootFlags) *cobra.Command {
	var flagLine string
	var flagDirection string

	cmd := &cobra.Command{
		Use:   "stops",
		Short: "Print the ordered stop list for a line and direction",
		Example: strings.Trim(`
  # List stops for tram line A, direction 0 (default)
  infotbm-pp-cli lines stops --line A

  # List stops for tram line A, direction 1 (return)
  infotbm-pp-cli lines stops --line A --direction 1

  # List stops for bus line 1
  infotbm-pp-cli lines stops --line 1
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch ordered stop list for line from estimated timetable")
				return nil
			}
			if flagLine == "" {
				return usageErr(fmt.Errorf("--line is required"))
			}

			dir := "0"
			if flagDirection != "" {
				if flagDirection != "0" && flagDirection != "1" {
					return usageErr(fmt.Errorf("--direction must be 0 or 1"))
				}
				dir = flagDirection
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

			// Fetch estimated timetable for the line and direction to extract stop sequence
			results, err := extractStopSequenceFromTimetable(cmd.Context(), c, fullLineRef, dir)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if len(results) == 0 {
				return fmt.Errorf("no stops found for line %q direction %s; the line may have no active journeys in the estimated timetable", flagLine, dir)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagLine, "line", "", "Line short name (required, e.g. A, B, 1)")
	cmd.Flags().StringVar(&flagDirection, "direction", "0", "Direction: 0 (outbound, default) or 1 (return)")
	return cmd
}

// extractStopSequenceFromTimetable fetches the estimated timetable for a line/direction
// and extracts the ordered stop sequence from the first vehicle journey's EstimatedCalls.
func extractStopSequenceFromTimetable(ctx context.Context, c *client.Client, lineRef, direction string) ([]lineStopView, error) {
	params := map[string]string{
		"LineRef":      lineRef,
		"DirectionRef": direction,
	}
	data, err := c.Get(ctx, "/siri/2.0/bordeaux/estimated-timetable.json", params)
	if err != nil {
		return nil, err
	}

	// Navigate SIRI envelope: Siri > ServiceDelivery > EstimatedTimetableDelivery > EstimatedJourneyVersionFrame > EstimatedVehicleJourney
	var journeys []map[string]any
	extractVehicleJourneys(data, &journeys)

	// Find the journey with the most stops — it represents a complete run
	var bestCalls []map[string]any
	for _, j := range journeys {
		calls := extractCallList(j)
		if len(calls) > len(bestCalls) {
			bestCalls = calls
		}
	}

	if len(bestCalls) == 0 {
		return nil, nil
	}

	// Deduplicate stops while preserving order (a journey visits each stop once)
	seen := make(map[string]bool)
	results := make([]lineStopView, 0, len(bestCalls))
	order := 1
	for _, call := range bestCalls {
		stopID := ""
		if v, ok := call["StopPointRef"].(string); ok {
			stopID = v
		} else if v, ok := call["StopPointRef"].(map[string]any); ok {
			if val, ok := v["value"].(string); ok {
				stopID = val
			}
		}
		if stopID == "" || seen[stopID] {
			continue
		}
		seen[stopID] = true

		stopName := ""
		for _, name := range extractSIRITextValues(call, "StopPointName") {
			stopName = name
			break
		}
		// Fallback to string form
		if stopName == "" {
			if v, ok := call["StopPointName"].(string); ok {
				stopName = v
			}
		}

		results = append(results, lineStopView{
			Order:    order,
			StopID:   stopID,
			StopName: stopName,
		})
		order++
	}

	return results, nil
}

// extractCallList extracts the EstimatedCall list from a vehicle journey.
func extractCallList(j map[string]any) []map[string]any {
	for _, callsKey := range []string{"EstimatedCalls", "estimatedCalls", "EstimatedCall", "RecordedCalls"} {
		calls, ok := j[callsKey]
		if !ok {
			continue
		}
		// Handle {"EstimatedCall": [...]} wrapper
		if cm, ok := calls.(map[string]any); ok {
			if inner, ok := cm["EstimatedCall"]; ok {
				calls = inner
			}
		}
		if arr, ok := calls.([]any); ok {
			result := make([]map[string]any, 0, len(arr))
			for _, c := range arr {
				if cm, ok := c.(map[string]any); ok {
					result = append(result, cm)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return nil
}
