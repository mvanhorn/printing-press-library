// Copyright 2026 jmbernabotto and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newJourneyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "journey",
		Short: "Journey planning and accessibility information",
	}
	cmd.AddCommand(newJourneyAccessibilityCmd(flags))
	return cmd
}

func newJourneyAccessibilityCmd(flags *rootFlags) *cobra.Command {
	var from, to, coverage, date string

	cmd := &cobra.Command{
		Use:   "accessibility",
		Short: "Show elevator and escalator status for each transfer on a journey",
		Long: `Plans a journey between two places, then queries /equipment_reports for
each transfer station to show elevator and escalator availability.

Useful for travellers with reduced mobility or those travelling with luggage.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli journey accessibility --from "stop_area:SNCF:87686006" --to "stop_area:SNCF:87723197"
  sncf-connect-pp-cli journey accessibility --from "stop_area:SNCF:87686006" --to "stop_area:SNCF:87723197" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if from == "" || to == "" {
				return fmt.Errorf("--from and --to are required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			journeyPath := fmt.Sprintf("/coverage/%s/journeys", coverage)
			journeyParams := map[string]string{
				"from": from,
				"to":   to,
			}
			if date != "" {
				journeyParams["datetime"] = date + "T080000"
			}

			journeyData, _, err := resolveRead(cmd.Context(), c, flags, "journeys", false, journeyPath, journeyParams, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse journeys response
			var journeyResp map[string]any
			if err := json.Unmarshal(journeyData, &journeyResp); err != nil {
				return fmt.Errorf("parsing journeys: %w", err)
			}

			journeys, _ := journeyResp["journeys"].([]any)
			if len(journeys) == 0 {
				var directList []map[string]any
				if json.Unmarshal(journeyData, &directList) == nil {
					for _, j := range directList {
						journeys = append(journeys, j)
					}
				}
			}

			if len(journeys) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No journeys found.")
				return nil
			}

			// Take the first (best) journey
			journey, _ := journeys[0].(map[string]any)

			// Collect unique transfer stop_point IDs
			transferStops := extractTransferStopPoints(journey)

			type legAccessibility struct {
				StopID      string   `json:"stop_id"`
				StopName    string   `json:"stop_name"`
				Elevators   []string `json:"elevators"`
				Escalators  []string `json:"escalators"`
				ElevatorOK  bool     `json:"elevator_ok"`
				EscalatorOK bool     `json:"escalator_ok"`
				NoEquipData bool     `json:"no_equip_data"`
			}

			var legs []legAccessibility

			for _, stop := range transferStops {
				eqPath := fmt.Sprintf("/coverage/%s/%s/equipment_reports", coverage, stop.id)
				eqData, _, eqErr := resolveRead(cmd.Context(), c, flags, "equipment_reports", false, eqPath, nil, nil)

				leg := legAccessibility{
					StopID:   stop.id,
					StopName: stop.name,
				}

				if eqErr != nil {
					leg.NoEquipData = true
				} else {
					parseEquipmentStatus(eqData, &leg.Elevators, &leg.Escalators, &leg.ElevatorOK, &leg.EscalatorOK)
				}
				legs = append(legs, leg)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"from":      from,
					"to":        to,
					"coverage":  coverage,
					"transfers": legs,
				})
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Journey accessibility: %s → %s\n\n", from, to)
			if len(legs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  No transfers — direct journey.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Transfer stations (%d):\n", len(legs))
			for _, leg := range legs {
				elevIcon := "✓"
				if !leg.ElevatorOK {
					elevIcon = "✗"
				}
				escalIcon := "✓"
				if !leg.EscalatorOK {
					escalIcon = "✗"
				}
				if leg.NoEquipData {
					elevIcon = "?"
					escalIcon = "?"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-40s  elevator:%s  escalator:%s\n",
					leg.StopName, elevIcon, escalIcon)
				if len(leg.Elevators) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    Elevators: %s\n", strings.Join(leg.Elevators, ", "))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Departure place (stop area URI, address, or city name)")
	cmd.Flags().StringVar(&to, "to", "", "Arrival place")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&date, "date", "", "Travel date in YYYYMMDD format (default: today)")
	return cmd
}

type stopRef struct {
	id   string
	name string
}

func extractTransferStopPoints(journey map[string]any) []stopRef {
	sections, _ := journey["sections"].([]any)
	seen := make(map[string]bool)
	var stops []stopRef

	for _, sec := range sections {
		s, _ := sec.(map[string]any)
		secType, _ := s["type"].(string)
		if secType != "transfer" && secType != "waiting" {
			continue
		}
		if from, ok := s["from"].(map[string]any); ok {
			if sa, ok := from["stop_area"].(map[string]any); ok {
				id, _ := sa["id"].(string)
				name, _ := sa["name"].(string)
				if id != "" && !seen[id] {
					seen[id] = true
					stops = append(stops, stopRef{id: "stop_area:" + strings.TrimPrefix(id, "stop_area:"), name: name})
				}
			}
		}
	}
	return stops
}

func parseEquipmentStatus(data json.RawMessage, elevators, escalators *[]string, elevOK, escalOK *bool) {
	var resp map[string]any
	if json.Unmarshal(data, &resp) != nil {
		return
	}

	reports, _ := resp["equipment_reports"].([]any)
	if len(reports) == 0 {
		var list []map[string]any
		if json.Unmarshal(data, &list) == nil {
			for _, r := range list {
				reports = append(reports, r)
			}
		}
	}

	elevSeen := false
	escalSeen := false

	for _, rep := range reports {
		r, _ := rep.(map[string]any)
		eqs, _ := r["equipment_details"].([]any)
		for _, eq := range eqs {
			e, _ := eq.(map[string]any)
			eqType, _ := e["embedded_type"].(string)
			status, _ := e["current_availability"].(map[string]any)
			effect, _ := status["status"].(string)

			name := ""
			if n, ok := e["name"].(string); ok {
				name = n
			} else if id, ok := e["id"].(string); ok {
				name = id
			}

			switch strings.ToLower(eqType) {
			case "elevator":
				*elevators = append(*elevators, name)
				if !elevSeen {
					*elevOK = true
					elevSeen = true
				}
				if strings.ToLower(effect) != "available" {
					*elevOK = false
				}
			case "escalator":
				*escalators = append(*escalators, name)
				if !escalSeen {
					*escalOK = true
					escalSeen = true
				}
				if strings.ToLower(effect) != "available" {
					*escalOK = false
				}
			}
		}
	}
}
