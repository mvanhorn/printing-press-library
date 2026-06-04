// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/store"
	"github.com/spf13/cobra"
)

func newNovelTripsPlanCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagDepart string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan a journey using local GTFS and live disruptions",
		Long: `Plan a journey between two stops using SIRI estimated timetable data.
Resolves stop names from the local GTFS store and searches live timetable
data for connections. Active alerts affecting the route are checked and
included in the output.`,
		Example: strings.Trim(`
  # Plan a trip from Quinconces to Pessac Centre departing at 08:15
  infotbm-pp-cli trips plan --from Quinconces --to "Pessac Centre" --depart 08:15

  # Plan a trip from Gare Saint-Jean to the airport
  infotbm-pp-cli trips plan --from "Gare Saint-Jean" --to "Aeroport" --depart 14:00
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would plan a journey using GTFS data and live disruptions")
				return nil
			}
			if flagFrom == "" {
				return usageErr(fmt.Errorf("--from is required"))
			}
			if flagTo == "" {
				return usageErr(fmt.Errorf("--to is required"))
			}
			if flagDepart == "" {
				return usageErr(fmt.Errorf("--depart is required (e.g. 08:15)"))
			}

			// Parse departure time
			departTime, err := time.Parse("15:04", flagDepart)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --depart time %q: use HH:MM format", flagDepart))
			}
			now := time.Now()
			departAt := time.Date(now.Year(), now.Month(), now.Day(),
				departTime.Hour(), departTime.Minute(), 0, 0, now.Location())

			// Resolve stop names from local store
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("infotbm-pp-cli")
			}
			st, stErr := store.OpenReadOnly(dbPath)
			if stErr != nil {
				return fmt.Errorf("opening local store: %w (run 'infotbm-pp-cli sync' first)", stErr)
			}
			defer st.Close()

			fromID, _ := st.ResolveByName("stops", flagFrom, "name", "stop_name")
			if fromID == "" {
				fromID = flagFrom
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

			// Fetch active alerts for disruption awareness
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

			// Find journeys connecting from -> to
			var journeys []map[string]any
			extractVehicleJourneys(data, &journeys)

			type journeyOption struct {
				Line          string    `json:"line"`
				DepartureTime string    `json:"departure_time"`
				ArrivalTime   string    `json:"arrival_time,omitempty"`
				Duration      string    `json:"duration,omitempty"`
				StopCount     int       `json:"stop_count"`
				FromStop      string    `json:"from_stop"`
				ToStop        string    `json:"to_stop"`
				Disruptions   []string  `json:"disruptions"`
				depTimeVal    time.Time `json:"-"`
			}

			options := make([]journeyOption, 0)
			for _, j := range journeys {
				lineRef := ""
				for _, key := range []string{"LineRef", "lineRef"} {
					if v, ok := j[key].(string); ok {
						lineRef = v
						break
					}
				}

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

				// Only include departures after the requested time
				if depTime.IsZero() || depTime.Before(departAt) {
					continue
				}

				lineName := lineRef
				parts := strings.Split(lineRef, ":")
				if len(parts) >= 4 {
					lineName = parts[len(parts)-2]
				} else if len(parts) > 0 {
					lineName = parts[len(parts)-1]
				}

				opt := journeyOption{
					Line:       lineName,
					FromStop:   flagFrom,
					ToStop:     flagTo,
					StopCount:  toIdx - fromIdx,
					depTimeVal: depTime,
				}
				if !depTime.IsZero() {
					opt.DepartureTime = depTime.Format(time.RFC3339)
				}
				if !arrTime.IsZero() {
					opt.ArrivalTime = arrTime.Format(time.RFC3339)
				}
				if !depTime.IsZero() && !arrTime.IsZero() {
					opt.Duration = arrTime.Sub(depTime).String()
				}

				// Check for disruptions on this line
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

			// Limit to top 5 options
			if len(options) > 5 {
				options = options[:5]
			}

			view := struct {
				From        string          `json:"from"`
				To          string          `json:"to"`
				DepartAfter string          `json:"depart_after"`
				Found       int             `json:"found"`
				Options     []journeyOption `json:"options"`
			}{
				From:        flagFrom,
				To:          flagTo,
				DepartAfter: departAt.Format(time.RFC3339),
				Found:       totalFound,
				Options:     options,
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Origin stop name (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Destination stop name (required)")
	cmd.Flags().StringVar(&flagDepart, "depart", "", "Departure time in HH:MM format (required, e.g. 08:15)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path override")
	return cmd
}

func matchStopName(name, query string) bool {
	if name == "" || query == "" {
		return false
	}
	return strings.EqualFold(name, query) || strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

func alertAffectsLine(alert map[string]any, lineRef string) bool {
	lineUpper := strings.ToUpper(lineRef)
	parts := strings.Split(lineRef, ":")
	short := parts[len(parts)-1]
	if len(parts) >= 4 {
		short = parts[len(parts)-2]
	}
	shortUpper := strings.ToUpper(short)

	for _, key := range []string{"AffectedLineRef", "LineRef", "affectedLines", "lines"} {
		v, ok := alert[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.ToUpper(val) == lineUpper || strings.ToUpper(val) == shortUpper {
				return true
			}
		case []any:
			for _, item := range val {
				if s, ok := item.(string); ok {
					if strings.ToUpper(s) == lineUpper || strings.ToUpper(s) == shortUpper {
						return true
					}
				}
			}
		}
	}
	return false
}
