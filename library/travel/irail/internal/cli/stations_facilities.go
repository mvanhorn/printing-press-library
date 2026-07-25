// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: stations facilities.
//
// Serves the open iRail facilities dataset. The API's own /v1/stations response
// carries only id, name, standardname and coordinates, so accessibility and
// amenity questions cannot be answered from the live API at all.
//
// pp:novel-static-reference

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/irailref"
)

type facilitiesView struct {
	Station        string                 `json:"station"`
	StandardName   string                 `json:"standard_name,omitempty"`
	ID             string                 `json:"id,omitempty"`
	TelegraphCode  string                 `json:"telegraph_code,omitempty"`
	Address        string                 `json:"address,omitempty"`
	StepFree       bool                   `json:"step_free"`
	Accessibility  map[string]bool        `json:"accessibility"`
	Services       map[string]bool        `json:"services"`
	Connections    map[string]bool        `json:"onward_connections"`
	DisabledParkig int                    `json:"disabled_parking_spots"`
	TransferSec    int                    `json:"official_transfer_seconds,omitempty"`
	SalesHours     []irailref.SalesWindow `json:"ticket_desk_hours,omitempty"`
	Note           string                 `json:"note,omitempty"`
}

func newNovelStationsFacilitiesCmd(flags *rootFlags) *cobra.Command {
	var flagStation string

	cmd := &cobra.Command{
		Use:   "facilities",
		Short: "Accessibility, amenities and ticket-desk hours for a station",
		Long: "Reports step-free access, elevators, ramps, lockers, bike parking, onward\n" +
			"transport and ticket-desk opening hours for a station.\n\n" +
			"Use this command for accessibility and amenity questions. Do NOT use it to find\n" +
			"a station by name; use 'stations search' for that.\n\n" +
			"Data comes from the open iRail facilities dataset bundled with this CLI, not\n" +
			"from the live API, which does not publish these fields.",
		Example: `  irail-pp-cli stations facilities --station Ghent-Sint-Pieters
  irail-pp-cli stations facilities --station FBMZ --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report station facilities from the bundled dataset")
				return nil
			}
			// Accept the station as a positional too, so `stations facilities Leuven` works.
			if flagStation == "" && len(args) > 0 {
				flagStation = args[0]
			}
			if flagStation == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--station is required"))
			}

			st, ok := irailref.Lookup(flagStation)
			if !ok {
				return notFoundErr(fmt.Errorf(
					"no station matches %q; try 'irail-pp-cli stations search %s'", flagStation, flagStation))
			}

			view := facilitiesView{
				Station:       st.Name,
				ID:            st.ID,
				TelegraphCode: st.Telegraph,
				Accessibility: map[string]bool{},
				Services:      map[string]bool{},
				Connections:   map[string]bool{},
			}
			if st.HasTransfer {
				view.TransferSec = st.TransferSeconds
			}

			f, ok := irailref.FacilitiesFor(st)
			if !ok {
				view.Note = "no facilities record is published for this station in the open dataset"
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", st.Name, view.Note)
				return nil
			}

			view.StandardName = f.Name
			view.StepFree = f.StepFree()
			view.DisabledParkig = f.DisabledParkingSpots
			view.SalesHours = f.SalesHours
			if f.Street != "" || f.City != "" {
				view.Address = fmt.Sprintf("%s, %s %s", f.Street, f.Zip, f.City)
			}
			view.Accessibility = map[string]bool{
				"wheelchair_available": f.WheelchairAvailable,
				"ramp":                 f.Ramp,
				"elevator_platform":    f.ElevatorPlatform,
				"elevated_platform":    f.ElevatedPlatform,
				"escalator_up":         f.EscalatorUp,
				"escalator_down":       f.EscalatorDown,
				"audio_induction_loop": f.AudioInductionLoop,
			}
			view.Services = map[string]bool{
				"ticket_vending_machine": f.TicketVendingMachine,
				"luggage_lockers":        f.LuggageLockers,
				"free_parking":           f.FreeParking,
				"taxi":                   f.Taxi,
				"bicycle_spots":          f.BicycleSpots,
				"blue_bike":              f.BlueBike,
			}
			view.Connections = map[string]bool{
				"bus":   f.Bus,
				"tram":  f.Tram,
				"metro": f.Metro,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s", st.Name)
			if st.Telegraph != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s]", st.Telegraph)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if view.Address != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Address)
			}
			if view.StepFree {
				fmt.Fprintln(cmd.OutOrStdout(), "\nStep-free access: yes")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\nStep-free access: not recorded")
			}
			printFlagMap(cmd, "Accessibility", view.Accessibility)
			printFlagMap(cmd, "Services", view.Services)
			printFlagMap(cmd, "Onward transport", view.Connections)
			if view.DisabledParkig > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDisabled parking spots: %d\n", view.DisabledParkig)
			}
			if view.TransferSec > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Official minimum transfer time: %s\n", humanDuration(view.TransferSec))
			}
			if len(view.SalesHours) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTicket desk:")
				for _, w := range view.SalesHours {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %s - %s\n", w.Day, w.Open, w.Close)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagStation, "station", "", "Station name, telegraph code or id")
	return cmd
}

// printFlagMap renders only the features a station actually has, so absence of
// data never reads as a definitive "no".
func printFlagMap(cmd *cobra.Command, title string, m map[string]bool) {
	var present []string
	for k, v := range m {
		if v {
			present = append(present, k)
		}
	}
	if len(present) == 0 {
		return
	}
	sortStrings(present)
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", title, present)
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
