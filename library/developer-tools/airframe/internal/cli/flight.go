// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/flightgoat"

	"github.com/spf13/cobra"
)

func newFlightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "flight <ident>",
		Short: "Resolve a commercial flight ident via flight-goat, then enrich with airframe data.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFlight(cmd.Context(), args[0])
		},
	}
}

type FlightDossier struct {
	Flight    *flightgoat.FlightLookup `json:"flight"`
	Aircraft  *AircraftRow             `json:"aircraft,omitempty"`
	MakeModel *MakeModelRow            `json:"make_model,omitempty"`
	Engine    *EngineRow               `json:"engine,omitempty"`
	History   []EventSummaryRow        `json:"history"`
}

func runFlight(ctx context.Context, ident string) error {
	if !flightgoat.IsInstalled() {
		return fmt.Errorf("flight-goat-pp-cli is not installed on your PATH. %s", flightgoat.InstallHint)
	}

	lookup, err := flightgoat.ResolveIdent(ctx, ident)
	if err != nil {
		if errors.Is(err, flightgoat.ErrNoRegistration) {
			return fmt.Errorf("flight-goat could not resolve %q to a tail number — try `flight-goat-pp-cli aircraft owner get-aircraft %s` directly to inspect the upstream response", ident, ident)
		}
		return err
	}

	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	dossier := &FlightDossier{Flight: lookup, History: []EventSummaryRow{}}

	dossier.Aircraft, _ = queryAircraft(ctx, st.DB(), lookup.Registration)
	if dossier.Aircraft != nil {
		if dossier.Aircraft.MakeModelCode != nil {
			dossier.MakeModel, _ = queryMakeModel(ctx, st.DB(), *dossier.Aircraft.MakeModelCode)
		}
		if dossier.Aircraft.EngineCode != nil {
			dossier.Engine, _ = queryEngine(ctx, st.DB(), *dossier.Aircraft.EngineCode)
		}
	}
	dossier.History, _ = queryHistoryByRegistration(ctx, st.DB(), lookup.Registration)

	env := Envelope{
		Meta: Meta{
			Source:   "local+flight-goat",
			DBPath:   dbPath,
			SyncedAt: latestSyncedAt(ctx, st),
			Query:    map[string]any{"ident": ident, "resolved_registration": lookup.Registration},
		},
		Results: dossier,
	}

	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderFlightText(dossier)
}

func renderFlightText(d *FlightDossier) error {
	fmt.Printf("Flight %s → tail %s  (resolved via %s)\n", d.Flight.Ident, d.Flight.Registration, d.Flight.Source)
	if d.Aircraft == nil {
		fmt.Println("\n(no FAA registry data for this tail — it may not be US-registered, or your local store may be stale)")
		return nil
	}
	fmt.Println()
	if d.MakeModel != nil {
		fmt.Printf("  Type:    %s %s\n", d.MakeModel.Manufacturer, d.MakeModel.Model)
	}
	if d.Aircraft.YearMfr != nil {
		fmt.Printf("  Year:    %d\n", *d.Aircraft.YearMfr)
	}
	if d.Aircraft.OwnerName != nil {
		fmt.Printf("  Owner:   %s\n", *d.Aircraft.OwnerName)
	}
	if d.Aircraft.ModeSCodeHex != nil {
		fmt.Printf("  Mode-S:  %s\n", *d.Aircraft.ModeSCodeHex)
	}
	fmt.Printf("\nHistory (%d NTSB events)\n", len(d.History))
	if len(d.History) == 0 {
		fmt.Println("  (no NTSB-investigated events found for this tail)")
		return nil
	}
	for _, e := range d.History {
		injury := ""
		if e.HighestInjury != nil {
			injury = " " + *e.HighestInjury
		}
		fmt.Printf("  %s  %s%s  %s\n", e.EventDate, e.EventID, injury, derefOrEmpty(e.Summary))
	}
	return nil
}
