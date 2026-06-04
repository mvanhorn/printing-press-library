// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/store"
	"github.com/spf13/cobra"
)

func newNovelScheduleDiffCmd(flags *rootFlags) *cobra.Command {
	var flagStop string
	var flagLine string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare GTFS schedule with live data to find ghost services",
		Long: `Compare scheduled GTFS trips against live SIRI estimated timetable data
to surface "ghost services" — trips that exist on paper but are absent
from real-time tracking. This helps identify scheduled services that
were silently cancelled or never dispatched.`,
		Example: strings.Trim(`
  # Find ghost services on tram line A
  infotbm-pp-cli schedule diff --line A

  # Find ghost services at a specific stop
  infotbm-pp-cli schedule diff --stop stop_point:BP:SA:3136

  # Find ghost services on line B at a specific stop
  infotbm-pp-cli schedule diff --line B --stop stop_point:BP:SA:3120
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare GTFS schedule with live SIRI data to find ghost services")
				return nil
			}
			if flagLine == "" && flagStop == "" {
				return usageErr(fmt.Errorf("at least one of --line or --stop is required"))
			}

			// Open local store for scheduled data
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("infotbm-pp-cli")
			}
			st, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w (run 'infotbm-pp-cli sync' first)", err)
			}
			defer st.Close()

			// Get scheduled routes from local store
			scheduledRoutes, err := st.List("routes", 0)
			if err != nil {
				return fmt.Errorf("reading scheduled routes: %w", err)
			}

			// Build set of scheduled trip/route references
			type scheduledTrip struct {
				RouteID   string `json:"route_id"`
				RouteName string `json:"route_name"`
				ShortName string `json:"short_name"`
			}
			scheduledByLine := make(map[string]scheduledTrip)
			for _, raw := range scheduledRoutes {
				var route map[string]any
				if json.Unmarshal(raw, &route) != nil {
					continue
				}
				sn := ""
				for _, key := range []string{"shortName", "short_name", "ShortName", "route_short_name"} {
					if v, ok := route[key].(string); ok {
						sn = v
						break
					}
				}
				id := ""
				for _, key := range []string{"id", "route_id", "routeId"} {
					if v, ok := route[key].(string); ok {
						id = v
						break
					}
				}
				name := ""
				for _, key := range []string{"longName", "long_name", "LongName", "route_long_name", "name"} {
					if v, ok := route[key].(string); ok {
						name = v
						break
					}
				}
				if sn != "" {
					scheduledByLine[strings.ToUpper(sn)] = scheduledTrip{
						RouteID:   id,
						RouteName: name,
						ShortName: sn,
					}
				}
			}

			// Fetch live estimated timetable
			c, cErr := flags.newClient()
			if cErr != nil {
				return cErr
			}

			params := map[string]string{}
			if flagLine != "" {
				params["LineRef"] = flagLine
			}
			liveData, err := c.Get(cmd.Context(), "/siri/2.0/bordeaux/estimated-timetable.json", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Extract live journey references
			var liveJourneys []map[string]any
			extractVehicleJourneys(liveData, &liveJourneys)

			// Filter by stop if requested
			if flagStop != "" {
				filtered := make([]map[string]any, 0)
				for _, j := range liveJourneys {
					calls := extractCallsList(j)
					for _, call := range calls {
						ref := callStopRef(call)
						if matchStopRef(ref, flagStop, flagStop) {
							filtered = append(filtered, j)
							break
						}
					}
				}
				liveJourneys = filtered
			}

			liveLineRefs := make(map[string]bool)
			liveTripRefs := make(map[string]bool)
			for _, j := range liveJourneys {
				for _, key := range []string{"LineRef", "lineRef"} {
					if v, ok := j[key].(string); ok {
						parts := strings.Split(v, ":")
						for _, p := range parts {
							liveLineRefs[strings.ToUpper(p)] = true
						}
						liveLineRefs[strings.ToUpper(v)] = true
					}
				}
				for _, key := range []string{"DatedVehicleJourneyRef", "datedVehicleJourneyRef", "FramedVehicleJourneyRef"} {
					if v, ok := j[key].(string); ok {
						liveTripRefs[v] = true
					}
					if vm, ok := j[key].(map[string]any); ok {
						if ref, ok := vm["DatedVehicleJourneyRef"].(string); ok {
							liveTripRefs[ref] = true
						}
					}
				}
			}

			// Compare: find scheduled lines with no live data
			type ghostService struct {
				LineShortName  string `json:"line_short_name"`
				RouteID        string `json:"route_id"`
				RouteName      string `json:"route_name"`
				Status         string `json:"status"`
				LiveDepartures int    `json:"live_departures"`
			}

			results := make([]ghostService, 0)
			lineUpper := strings.ToUpper(flagLine)

			for sn, trip := range scheduledByLine {
				if flagLine != "" && sn != lineUpper {
					continue
				}
				if !liveLineRefs[sn] {
					results = append(results, ghostService{
						LineShortName:  trip.ShortName,
						RouteID:        trip.RouteID,
						RouteName:      trip.RouteName,
						Status:         "ghost",
						LiveDepartures: 0,
					})
				} else {
					// Count live departures for this line
					count := 0
					for _, j := range liveJourneys {
						for _, key := range []string{"LineRef", "lineRef"} {
							if v, ok := j[key].(string); ok {
								parts := strings.Split(v, ":")
								linePart := parts[len(parts)-1]
								if len(parts) >= 4 {
									linePart = parts[len(parts)-2]
								}
								if strings.ToUpper(linePart) == sn {
									count++
								}
							}
						}
					}
					results = append(results, ghostService{
						LineShortName:  trip.ShortName,
						RouteID:        trip.RouteID,
						RouteName:      trip.RouteName,
						Status:         "active",
						LiveDepartures: count,
					})
				}
			}

			view := struct {
				Line           string         `json:"line,omitempty"`
				Stop           string         `json:"stop,omitempty"`
				ScheduledCount int            `json:"scheduled_count"`
				LiveCount      int            `json:"live_journey_count"`
				GhostCount     int            `json:"ghost_count"`
				Services       []ghostService `json:"services"`
			}{
				Line:           flagLine,
				Stop:           flagStop,
				ScheduledCount: len(scheduledByLine),
				LiveCount:      len(liveJourneys),
				Services:       results,
			}

			ghosts := 0
			for _, s := range results {
				if s.Status == "ghost" {
					ghosts++
				}
			}
			view.GhostCount = ghosts

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagStop, "stop", "", "Stop code to filter by")
	cmd.Flags().StringVar(&flagLine, "line", "", "Line short name to filter by")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path override")
	return cmd
}
