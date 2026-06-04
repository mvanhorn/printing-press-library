// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/store"
	"github.com/spf13/cobra"
)

func newNovelTripsRerouteCmd(flags *rootFlags) *cobra.Command {
	var flagAt string
	var flagTo string
	var flagDelay string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "reroute",
		Short: "Find alternate routes when current connection is delayed",
		Long: `When your current connection is delayed, find alternate onward routes
to your destination. Factors in the delay at your current position and
searches for the next best connections using live SIRI estimated timetable data.`,
		Example: strings.Trim(`
  # Find alternatives when delayed 10 minutes at Quinconces heading to Pessac
  infotbm-pp-cli trips reroute --at Quinconces --to "Pessac Centre" --delay 10

  # Reroute from Gare Saint-Jean with 15 minute delay
  infotbm-pp-cli trips reroute --at "Gare Saint-Jean" --to "Meriadeck" --delay 15
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find alternate routes factoring in current delay")
				return nil
			}
			if flagAt == "" {
				return usageErr(fmt.Errorf("--at is required"))
			}
			if flagTo == "" {
				return usageErr(fmt.Errorf("--to is required"))
			}
			if flagDelay == "" {
				return usageErr(fmt.Errorf("--delay is required (minutes)"))
			}

			delayMins, err := strconv.Atoi(flagDelay)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --delay value %q: must be integer minutes", flagDelay))
			}

			// Earliest usable time is now + delay
			now := time.Now()
			earliest := now.Add(time.Duration(delayMins) * time.Minute)

			// Resolve stop names
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("infotbm-pp-cli")
			}
			st, stErr := store.OpenReadOnly(dbPath)
			if stErr != nil {
				return fmt.Errorf("opening local store: %w (run 'infotbm-pp-cli sync' first)", stErr)
			}
			defer st.Close()

			atID, _ := st.ResolveByName("stops", flagAt, "name", "stop_name")
			if atID == "" {
				atID = flagAt
			}
			toID, _ := st.ResolveByName("stops", flagTo, "name", "stop_name")
			if toID == "" {
				toID = flagTo
			}

			c, cErr := flags.newClient()
			if cErr != nil {
				return cErr
			}

			// Fetch estimated timetable
			data, err := c.Get(cmd.Context(), "/siri/2.0/bordeaux/estimated-timetable.json", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Fetch alerts for disruption info
			alertData, alertErr := c.Get(cmd.Context(), "/alerts/active/bordeaux", nil)
			var activeAlerts []map[string]any
			if alertErr == nil {
				var rawAlerts []json.RawMessage
				if json.Unmarshal(alertData, &rawAlerts) == nil {
					for _, raw := range rawAlerts {
						var a map[string]any
						if json.Unmarshal(raw, &a) == nil {
							activeAlerts = append(activeAlerts, a)
						}
					}
				}
			}

			var journeys []map[string]any
			extractVehicleJourneys(data, &journeys)

			type rerouteOption struct {
				Line          string    `json:"line"`
				DepartureTime string    `json:"departure_time"`
				ArrivalTime   string    `json:"arrival_time,omitempty"`
				Duration      string    `json:"duration,omitempty"`
				StopCount     int       `json:"stop_count"`
				WaitMins      float64   `json:"wait_minutes"`
				Disruptions   []string  `json:"disruptions"`
				depTimeVal    time.Time `json:"-"`
			}

			options := make([]rerouteOption, 0)
			for _, j := range journeys {
				lineRef := ""
				for _, key := range []string{"LineRef", "lineRef"} {
					if v, ok := j[key].(string); ok {
						lineRef = v
						break
					}
				}

				calls := extractCallsList(j)
				atIdx := -1
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
					if matchStopRef(stopRef, atID, flagAt) || matchStopName(stopName, flagAt) {
						atIdx = i
					}
					if atIdx >= 0 && (matchStopRef(stopRef, toID, flagTo) || matchStopName(stopName, flagTo)) {
						toIdx = i
						break
					}
				}

				if atIdx < 0 || toIdx < 0 || toIdx <= atIdx {
					continue
				}

				depTime := callDepartureTime(calls[atIdx])
				arrTime := callArrivalTime(calls[toIdx])

				// Only include departures after earliest usable time
				if depTime.IsZero() || depTime.Before(earliest) {
					continue
				}

				lineName := lineRef
				parts := strings.Split(lineRef, ":")
				if len(parts) >= 4 {
					lineName = parts[len(parts)-2]
				} else if len(parts) > 0 {
					lineName = parts[len(parts)-1]
				}

				opt := rerouteOption{
					Line:      lineName,
					StopCount: toIdx - atIdx,
				}
				if !depTime.IsZero() {
					opt.DepartureTime = depTime.Format(time.RFC3339)
					opt.WaitMins = depTime.Sub(now).Minutes()
					opt.depTimeVal = depTime
				}
				if !arrTime.IsZero() {
					opt.ArrivalTime = arrTime.Format(time.RFC3339)
				}
				if !depTime.IsZero() && !arrTime.IsZero() {
					opt.Duration = arrTime.Sub(depTime).String()
				}

				// Check disruptions
				disruptions := make([]string, 0)
				for _, alert := range activeAlerts {
					if alertAffectsLine(alert, lineRef) {
						desc := ""
						for _, key := range []string{"Summary", "title", "Description", "description"} {
							if v, ok := alert[key].(string); ok && v != "" {
								desc = v
								break
							}
						}
						if desc != "" {
							disruptions = append(disruptions, desc)
						}
					}
				}
				opt.Disruptions = disruptions

				options = append(options, opt)
			}

			// Sort by departure time
			sort.Slice(options, func(i, j int) bool {
				return options[i].depTimeVal.Before(options[j].depTimeVal)
			})

			totalFound := len(options)

			// Limit to top 5
			if len(options) > 5 {
				options = options[:5]
			}

			view := struct {
				CurrentStop string          `json:"current_stop"`
				Destination string          `json:"destination"`
				DelayMins   int             `json:"delay_minutes"`
				EarliestAt  string          `json:"earliest_departure"`
				Found       int             `json:"found"`
				Options     []rerouteOption `json:"options"`
			}{
				CurrentStop: flagAt,
				Destination: flagTo,
				DelayMins:   delayMins,
				EarliestAt:  earliest.Format(time.RFC3339),
				Found:       totalFound,
				Options:     options,
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagAt, "at", "", "Current stop name (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination stop name (required)")
	cmd.Flags().StringVar(&flagDelay, "delay", "", "Current delay in minutes (required)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path override")
	return cmd
}
