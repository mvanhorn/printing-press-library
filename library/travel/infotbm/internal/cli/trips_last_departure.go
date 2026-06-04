// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/store"
	"github.com/spf13/cobra"
)

func newNovelTripsLastDepartureCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagBefore string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "last-departure",
		Short: "Find the latest departure that reaches destination before a cutoff time",
		Example: strings.Trim(`
  # Find last departure from Quinconces to Merignac Centre before 23:30
  infotbm-pp-cli trips last-departure --from Quinconces --to "Merignac Centre" --before 23:30

  # Find last departure from Gare Saint-Jean to Pessac Centre before midnight
  infotbm-pp-cli trips last-departure --from "Gare Saint-Jean" --to "Pessac Centre" --before 00:00
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find the latest departure reaching destination before cutoff")
				return nil
			}
			if flagFrom == "" {
				return usageErr(fmt.Errorf("--from is required"))
			}
			if flagTo == "" {
				return usageErr(fmt.Errorf("--to is required"))
			}
			if flagBefore == "" {
				return usageErr(fmt.Errorf("--before is required (e.g. 23:30)"))
			}

			// Parse cutoff time
			beforeTime, err := time.Parse("15:04", flagBefore)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --before time %q: use HH:MM format", flagBefore))
			}
			now := time.Now()
			cutoff := time.Date(now.Year(), now.Month(), now.Day(),
				beforeTime.Hour(), beforeTime.Minute(), 0, 0, now.Location())
			// If cutoff is before now, assume next day
			if cutoff.Before(now) {
				cutoff = cutoff.Add(24 * time.Hour)
			}

			// Resolve stop names to IDs from local store
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("infotbm-pp-cli")
			}
			st, stErr := store.OpenReadOnly(dbPath)
			if stErr != nil {
				return fmt.Errorf("opening local store: %w (run 'infotbm-pp-cli sync' first)", stErr)
			}
			defer st.Close()

			fromID, err := st.ResolveByName("stops", flagFrom, "name", "stop_name")
			if err != nil {
				// Fallback: use the raw input as ID
				fromID = flagFrom
			}
			toID, err := st.ResolveByName("stops", flagTo, "name", "stop_name")
			if err != nil {
				toID = flagTo
			}

			// Fetch estimated timetable to find departures
			c, cErr := flags.newClient()
			if cErr != nil {
				return cErr
			}

			data, err := c.Get(cmd.Context(), "/siri/2.0/bordeaux/estimated-timetable.json", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse journeys and find those serving from->to
			var journeys []map[string]any
			extractVehicleJourneys(data, &journeys)

			type departureCandidate struct {
				Line          string    `json:"line"`
				DepartureTime string    `json:"departure_time"`
				ArrivalTime   string    `json:"arrival_time,omitempty"`
				FromStop      string    `json:"from_stop"`
				ToStop        string    `json:"to_stop"`
				JourneyRef    string    `json:"journey_ref,omitempty"`
				depTimeVal    time.Time `json:"-"`
			}

			candidates := make([]departureCandidate, 0)
			for _, j := range journeys {
				lineRef := ""
				for _, key := range []string{"LineRef", "lineRef"} {
					if v, ok := j[key].(string); ok {
						lineRef = v
						break
					}
				}
				journeyRef := ""
				for _, key := range []string{"DatedVehicleJourneyRef", "datedVehicleJourneyRef"} {
					if v, ok := j[key].(string); ok {
						journeyRef = v
						break
					}
				}

				// Extract calls to find from/to stops
				calls := extractCallsList(j)
				fromIdx := -1
				toIdx := -1
				for i, call := range calls {
					stopRef := callStopRef(call)
					stopName := ""
					for _, key := range []string{"StopPointName", "stopPointName", "StopName"} {
						if v, ok := call[key].(string); ok {
							stopName = v
							break
						}
					}
					if matchStopRef(stopRef, fromID, flagFrom) || matchStopName(stopName, flagFrom) {
						fromIdx = i
					}
					if fromIdx >= 0 && (matchStopRef(stopRef, toID, flagTo) || matchStopName(stopName, flagTo)) {
						toIdx = i
						break
					}
				}

				if fromIdx < 0 || toIdx < 0 || toIdx <= fromIdx {
					continue
				}

				depTime := callDepartureTime(calls[fromIdx])
				arrTime := callArrivalTime(calls[toIdx])

				if depTime.IsZero() {
					continue
				}

				// Skip if arrival time is unknown or after cutoff
				if arrTime.IsZero() || arrTime.After(cutoff) {
					continue
				}

				lineName := lineRef
				parts := strings.Split(lineRef, ":")
				if len(parts) >= 4 {
					lineName = parts[len(parts)-2]
				} else if len(parts) > 0 {
					lineName = parts[len(parts)-1]
				}

				candidates = append(candidates, departureCandidate{
					Line:          lineName,
					DepartureTime: depTime.Format(time.RFC3339),
					ArrivalTime: func() string {
						if arrTime.IsZero() {
							return ""
						}
						return arrTime.Format(time.RFC3339)
					}(),
					FromStop:   flagFrom,
					ToStop:     flagTo,
					JourneyRef: journeyRef,
					depTimeVal: depTime,
				})
			}

			// Sort by departure time descending to find the latest
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].depTimeVal.After(candidates[j].depTimeVal)
			})

			view := struct {
				From       string               `json:"from"`
				To         string               `json:"to"`
				Before     string               `json:"before"`
				Found      int                  `json:"found"`
				Latest     *departureCandidate  `json:"latest,omitempty"`
				Candidates []departureCandidate `json:"candidates"`
			}{
				From:       flagFrom,
				To:         flagTo,
				Before:     flagBefore,
				Found:      len(candidates),
				Candidates: candidates,
			}
			if len(candidates) > 0 {
				view.Latest = &candidates[0]
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin stop name (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination stop name (required)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Cutoff time in HH:MM format (required, e.g. 23:30)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path override")
	return cmd
}

// extractCallsList gets the ordered list of stop calls from a vehicle journey.
func extractCallsList(journey map[string]any) []map[string]any {
	var calls []map[string]any
	for _, key := range []string{"EstimatedCalls", "estimatedCalls", "EstimatedCall", "RecordedCalls", "Calls"} {
		if v, ok := journey[key]; ok {
			switch val := v.(type) {
			case []any:
				for _, c := range val {
					if cm, ok := c.(map[string]any); ok {
						calls = append(calls, cm)
					}
				}
			case map[string]any:
				// Sometimes nested under another key
				if inner, ok := val["EstimatedCall"].([]any); ok {
					for _, c := range inner {
						if cm, ok := c.(map[string]any); ok {
							calls = append(calls, cm)
						}
					}
				} else {
					calls = append(calls, val)
				}
			}
			if len(calls) > 0 {
				break
			}
		}
	}
	return calls
}

func callStopRef(call map[string]any) string {
	for _, key := range []string{"StopPointRef", "stopPointRef", "stop_point_ref", "StopRef"} {
		if v, ok := call[key].(string); ok {
			return v
		}
	}
	return ""
}

func matchStopRef(ref, id, name string) bool {
	if ref == "" {
		return false
	}
	if ref == id {
		return true
	}
	// Fuzzy match on name in the ref
	nameLower := strings.ToLower(name)
	refLower := strings.ToLower(ref)
	if strings.Contains(refLower, nameLower) {
		return true
	}
	// Check StopName field match
	return false
}

func callDepartureTime(call map[string]any) time.Time {
	for _, key := range []string{
		"ExpectedDepartureTime", "expectedDepartureTime",
		"AimedDepartureTime", "aimedDepartureTime",
	} {
		if ts, ok := call[key].(string); ok && ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func callArrivalTime(call map[string]any) time.Time {
	for _, key := range []string{
		"ExpectedArrivalTime", "expectedArrivalTime",
		"AimedArrivalTime", "aimedArrivalTime",
	} {
		if ts, ok := call[key].(string); ok && ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
