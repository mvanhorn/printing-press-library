// Copyright 2026 jmbernabotto. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newVehicleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vehicle",
		Short: "Vehicle journey queries",
	}
	cmd.AddCommand(newVehicleTraceCmd(flags))
	return cmd
}

func newVehicleTraceCmd(flags *rootFlags) *cobra.Command {
	var lineURI, coverage, date string
	var count int

	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Show the full ordered stop sequence for a vehicle journey (train's perspective)",
		Long: `Fetches all vehicle journeys for a line and shows their ordered stop
sequences with arrival and departure times. This is the train's perspective:
every stop the train makes, in order, with scheduled times.

Useful for populating schedule databases or understanding a line's full path.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli vehicle trace --line "line:SNCF:D"
  sncf-connect-pp-cli vehicle trace --line "line:SNCF:D" --date 20260601 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if lineURI == "" {
				return fmt.Errorf("--line is required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if date == "" {
				date = time.Now().Format("20060102")
			}

			path := fmt.Sprintf("/coverage/%s/lines/%s/vehicle_journeys", coverage, lineURI)
			params := map[string]string{
				"depth": "2",
				"count": fmt.Sprintf("%d", count),
				"since": date + "T000000",
				"until": date + "T235959",
			}

			data, _, err := resolveRead(cmd.Context(), c, flags, "vehicle_journeys", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			vjs := navitiaItems(data, "vehicle_journeys")

			type stopTime struct {
				Order       int    `json:"order"`
				StopID      string `json:"stop_id"`
				StopName    string `json:"stop_name"`
				ArrivalTime string `json:"arrival_time"`
				DepartureTime string `json:"departure_time"`
			}

			type vjTrace struct {
				ID        string     `json:"id"`
				Name      string     `json:"name"`
				Headsign  string     `json:"headsign"`
				StopTimes []stopTime `json:"stop_times"`
			}

			var traces []vjTrace

			for _, vj := range vjs {
				t := vjTrace{
					ID:       getString(vj, "id"),
					Name:     getString(vj, "name"),
					Headsign: getString(vj, "headsign"),
				}

				stopTimes, _ := vj["stop_times"].([]any)
				for i, st := range stopTimes {
					s, _ := st.(map[string]any)
					sp, _ := s["stop_point"].(map[string]any)
					stopID := ""
					stopName := ""
					if sp != nil {
						stopID, _ = sp["id"].(string)
						stopName, _ = sp["name"].(string)
					}
					arr := formatHHMM(getString(s, "arrival_time"))
					dep := formatHHMM(getString(s, "departure_time"))
					t.StopTimes = append(t.StopTimes, stopTime{
						Order:         i + 1,
						StopID:        stopID,
						StopName:      stopName,
						ArrivalTime:   arr,
						DepartureTime: dep,
					})
				}
				traces = append(traces, t)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"line":             lineURI,
					"coverage":         coverage,
					"date":             date,
					"vehicle_journeys": traces,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Vehicle journeys on %s (%s)\n\n", lineURI, coverage)

			if len(traces) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No vehicle journeys found.")
				return nil
			}

			for _, t := range traces {
				fmt.Fprintf(cmd.OutOrStdout(), "Journey: %s  (%s)\n", t.Name, t.Headsign)
				fmt.Fprintf(cmd.OutOrStdout(), "  %-4s  %-6s  %-6s  %s\n", "#", "ARR", "DEP", "STOP")
				for _, st := range t.StopTimes {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-4d  %-6s  %-6s  %s\n",
						st.Order, st.ArrivalTime, st.DepartureTime, st.StopName)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&lineURI, "line", "", "Line URI (e.g. line:OCE:TGV)")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&date, "date", "", "Date in YYYYMMDD format (default: today)")
	cmd.Flags().IntVar(&count, "count", 100, "Maximum number of vehicle journeys to fetch")
	return cmd
}

func formatHHMM(t string) string {
	// Navitia uses seconds-since-midnight: "85200" or HH:MM:SS
	if len(t) >= 5 && t[2] == ':' {
		return t[:5]
	}
	return t
}
