// Copyright 2026 jmbernabotto. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newBoardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Departure board commands",
	}
	cmd.AddCommand(newBoardExportCmd(flags))
	return cmd
}

func newBoardExportCmd(flags *rootFlags) *cobra.Command {
	var station, coverage, format string
	var count int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a full departure board as CSV or JSONL for pipeline ingestion",
		Long: `Fetches a departure board for a station and emits all rows as flat
CSV or newline-delimited JSON (JSONL). Each row contains: datetime, line code,
direction, vehicle_journey_id, headsign, and data_freshness.

Designed for ETL pipelines that need structured data without human-formatted output.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli board export --station "stop_area:SNCF:87686006"
  sncf-connect-pp-cli board export --station "stop_area:SNCF:87686006" --format jsonl --count 200
  sncf-connect-pp-cli board export --station "stop_area:SNCF:87686006" --format csv > departures.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if station == "" {
				return fmt.Errorf("--station is required (stop area URI)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/coverage/%s/stop_areas/%s/departures", coverage, station)
			params := map[string]string{
				"count":          fmt.Sprintf("%d", count),
				"data_freshness": "realtime",
			}

			data, _, err := resolveRead(cmd.Context(), c, flags, "departures", true, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			departures := navitiaItems(data, "departures")

			type flatRow struct {
				DateTime         string `json:"datetime"`
				BaseDateTime     string `json:"base_datetime"`
				DataFreshness    string `json:"data_freshness"`
				LineCode         string `json:"line_code"`
				LineName         string `json:"line_name"`
				NetworkName      string `json:"network_name"`
				Direction        string `json:"direction"`
				Headsign         string `json:"headsign"`
				VehicleJourneyID string `json:"vehicle_journey_id"`
				StopID           string `json:"stop_id"`
				StopName         string `json:"stop_name"`
			}

			var rows []flatRow
			for _, dep := range departures {
				row := flatRow{}

				if sdt, ok := dep["stop_date_time"].(map[string]any); ok {
					row.DateTime, _ = sdt["departure_date_time"].(string)
					row.BaseDateTime, _ = sdt["base_departure_date_time"].(string)
					row.DataFreshness, _ = sdt["data_freshness"].(string)
				}
				if di, ok := dep["display_informations"].(map[string]any); ok {
					row.LineCode, _ = di["code"].(string)
					row.LineName, _ = di["label"].(string)
					row.NetworkName, _ = di["network"].(string)
					row.Direction, _ = di["direction"].(string)
					row.Headsign, _ = di["headsign"].(string)
				}
				if links, ok := dep["links"].([]any); ok {
					for _, lnk := range links {
						l, _ := lnk.(map[string]any)
						if t, _ := l["type"].(string); t == "vehicle_journey" {
							row.VehicleJourneyID, _ = l["id"].(string)
						}
					}
				}
				if sp, ok := dep["stop_point"].(map[string]any); ok {
					row.StopID, _ = sp["id"].(string)
					row.StopName, _ = sp["name"].(string)
				}

				rows = append(rows, row)
			}

			w := cmd.OutOrStdout()

			switch {
			case flags.asJSON:
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"station": station, "count": len(rows), "departures": rows})
			case format == "jsonl":
				enc := json.NewEncoder(w)
				for _, row := range rows {
					if err := enc.Encode(row); err != nil {
						return err
					}
				}
			default: // csv
				cw := csv.NewWriter(w)
				headers := []string{"datetime", "base_datetime", "data_freshness", "line_code", "line_name", "network_name", "direction", "headsign", "vehicle_journey_id", "stop_id", "stop_name"}
				if err := cw.Write(headers); err != nil {
					return err
				}
				for _, row := range rows {
					record := []string{
						row.DateTime, row.BaseDateTime, row.DataFreshness,
						row.LineCode, row.LineName, row.NetworkName,
						row.Direction, row.Headsign, row.VehicleJourneyID,
						row.StopID, row.StopName,
					}
					if err := cw.Write(record); err != nil {
						return err
					}
				}
				cw.Flush()
				if err := cw.Error(); err != nil {
					return err
				}
			}

			if !flags.asJSON && format != "jsonl" {
				fmt.Fprintf(os.Stderr, "Exported %d departures from %s.\n", len(rows), station)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&station, "station", "", "Stop area URI (e.g. stop_area:OCE:SA:87391003)")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv or jsonl")
	cmd.Flags().IntVar(&count, "count", 100, "Maximum number of departures to fetch")
	return cmd
}
