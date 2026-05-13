// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
)

func newEventCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "event <event-id>",
		Short: "Single NTSB event with all aircraft involved and full narrative when available.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvent(cmd.Context(), args[0])
		},
	}
}

type EventDetailRow struct {
	EventID        string   `json:"event_id"`
	EventType      *string  `json:"event_type,omitempty"`
	EventDate      string   `json:"event_date"`
	EventCity      *string  `json:"event_city,omitempty"`
	EventState     *string  `json:"event_state,omitempty"`
	EventCountry   *string  `json:"event_country,omitempty"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	HighestInjury  *string  `json:"highest_injury,omitempty"`
	TotalFatal     *int     `json:"total_fatal,omitempty"`
	TotalSerious   *int     `json:"total_serious,omitempty"`
	TotalMinor     *int     `json:"total_minor,omitempty"`
	TotalUninjured *int     `json:"total_uninjured,omitempty"`
	Weather        *string  `json:"weather,omitempty"`
	LightCondition *string  `json:"light_condition,omitempty"`
	PhaseOfFlight  *string  `json:"phase_of_flight,omitempty"`
	NTSBReportNo   *string  `json:"ntsb_report_no,omitempty"`
}

type EventAircraftDetail struct {
	AircraftIdx  int     `json:"aircraft_idx"`
	Registration *string `json:"registration,omitempty"`
	Damage       *string `json:"damage,omitempty"`
	OperatorName *string `json:"operator_name,omitempty"`
	FARPart      *string `json:"far_part,omitempty"`
	FlightPhase  *string `json:"flight_phase,omitempty"`
	FAALinked    bool    `json:"faa_linked"`
	Manufacturer *string `json:"manufacturer,omitempty"`
	Model        *string `json:"model,omitempty"`
	YearMfr      *int    `json:"year_mfr,omitempty"`
	OwnerName    *string `json:"owner_name,omitempty"`
}

type EventDossier struct {
	Event     *EventDetailRow       `json:"event,omitempty"`
	Aircraft  []EventAircraftDetail `json:"aircraft"`
	Summary   *string               `json:"summary,omitempty"`
	Narrative *string               `json:"narrative,omitempty"`
}

func runEvent(ctx context.Context, eventID string) error {
	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return fmt.Errorf("event id must not be empty")
	}

	dossier := &EventDossier{Aircraft: []EventAircraftDetail{}}

	dossier.Event, err = queryEvent(ctx, st.DB(), eventID)
	if err != nil {
		return err
	}
	if dossier.Event == nil {
		return fmt.Errorf("no NTSB event found with id %s — try `airframe-pp-cli sync --source ntsb` if your store is stale", eventID)
	}
	dossier.Aircraft, err = queryEventAircraft(ctx, st.DB(), eventID)
	if err != nil {
		return err
	}
	dossier.Summary, dossier.Narrative, err = queryNarrative(ctx, st.DB(), eventID)
	if err != nil {
		return err
	}

	env := Envelope{
		Meta: Meta{
			Source: "local", DBPath: dbPath, SyncedAt: latestSyncedAt(ctx, st),
			Query: map[string]any{"event_id": eventID},
		},
		Results: dossier,
	}
	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderEventText(dossier)
}

func queryEvent(ctx context.Context, db *sql.DB, eventID string) (*EventDetailRow, error) {
	r := db.QueryRowContext(ctx, `SELECT event_id, event_type, event_date, event_city, event_state, event_country,
		latitude, longitude, highest_injury, total_fatal, total_serious, total_minor, total_uninjured,
		weather, light_condition, phase_of_flight, ntsb_report_no
		FROM events WHERE event_id = ?`, eventID)
	var e EventDetailRow
	var typ, city, state, country, inj, weather, light, phase, ntsb sql.NullString
	var lat, lng sql.NullFloat64
	var fatal, ser, minor, uni sql.NullInt64
	if err := r.Scan(&e.EventID, &typ, &e.EventDate, &city, &state, &country,
		&lat, &lng, &inj, &fatal, &ser, &minor, &uni,
		&weather, &light, &phase, &ntsb); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query event %s: %w", eventID, err)
	}
	e.EventType = nullToPtr(typ)
	e.EventCity = nullToPtr(city)
	e.EventState = nullToPtr(state)
	e.EventCountry = nullToPtr(country)
	if lat.Valid {
		v := lat.Float64
		e.Latitude = &v
	}
	if lng.Valid {
		v := lng.Float64
		e.Longitude = &v
	}
	e.HighestInjury = nullToPtr(inj)
	if fatal.Valid {
		v := int(fatal.Int64)
		e.TotalFatal = &v
	}
	if ser.Valid {
		v := int(ser.Int64)
		e.TotalSerious = &v
	}
	if minor.Valid {
		v := int(minor.Int64)
		e.TotalMinor = &v
	}
	if uni.Valid {
		v := int(uni.Int64)
		e.TotalUninjured = &v
	}
	e.Weather = nullToPtr(weather)
	e.LightCondition = nullToPtr(light)
	e.PhaseOfFlight = nullToPtr(phase)
	e.NTSBReportNo = nullToPtr(ntsb)
	return &e, nil
}

func queryEventAircraft(ctx context.Context, db *sql.DB, eventID string) ([]EventAircraftDetail, error) {
	const q = `SELECT ea.aircraft_idx, ea.registration, ea.damage, ea.operator_name, ea.far_part, ea.flight_phase,
		a.year_mfr, a.owner_name, mm.manufacturer, mm.model
	FROM event_aircraft ea
	LEFT JOIN aircraft a ON a.registration = ea.registration
	LEFT JOIN make_model mm ON mm.code = a.make_model_code
	WHERE ea.event_id = ?
	ORDER BY ea.aircraft_idx`
	rows, err := db.QueryContext(ctx, q, eventID)
	if err != nil {
		return nil, fmt.Errorf("query event_aircraft for %s: %w", eventID, err)
	}
	defer rows.Close()
	var out []EventAircraftDetail
	for rows.Next() {
		var a EventAircraftDetail
		var reg, damage, oper, far, phase, owner, mfr, model sql.NullString
		var year sql.NullInt64
		if err := rows.Scan(&a.AircraftIdx, &reg, &damage, &oper, &far, &phase, &year, &owner, &mfr, &model); err != nil {
			return nil, err
		}
		a.Registration = nullToPtr(reg)
		a.Damage = nullToPtr(damage)
		a.OperatorName = nullToPtr(oper)
		a.FARPart = nullToPtr(far)
		a.FlightPhase = nullToPtr(phase)
		if year.Valid {
			v := int(year.Int64)
			a.YearMfr = &v
			a.FAALinked = true
		}
		a.OwnerName = nullToPtr(owner)
		a.Manufacturer = nullToPtr(mfr)
		a.Model = nullToPtr(model)
		if mfr.Valid {
			a.FAALinked = true
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// queryNarrative returns (summary, full-narrative-when-stored). Full narrative
// is zstd-decompressed when present; otherwise nil. Both fields may be nil.
func queryNarrative(ctx context.Context, db *sql.DB, eventID string) (*string, *string, error) {
	r := db.QueryRowContext(ctx, `SELECT summary, full_zstd FROM narratives WHERE event_id = ?`, eventID)
	var summary sql.NullString
	var blob []byte
	if err := r.Scan(&summary, &blob); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("query narratives for %s: %w", eventID, err)
	}
	var sum *string
	if summary.Valid {
		v := summary.String
		sum = &v
	}
	var full *string
	if len(blob) > 0 {
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return sum, nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer dec.Close()
		var buf bytes.Buffer
		dec.Reset(bytes.NewReader(blob))
		if _, err := buf.ReadFrom(dec); err != nil {
			return sum, nil, fmt.Errorf("zstd decompress: %w", err)
		}
		s := buf.String()
		full = &s
	}
	return sum, full, nil
}

func renderEventText(d *EventDossier) error {
	e := d.Event
	fmt.Printf("NTSB event %s\n", e.EventID)
	fmt.Printf("  Date:        %s\n", e.EventDate)
	if e.EventCity != nil || e.EventState != nil {
		fmt.Printf("  Location:    %s, %s %s\n",
			derefOrEmpty(e.EventCity), derefOrEmpty(e.EventState), derefOrEmpty(e.EventCountry))
	}
	if e.HighestInjury != nil {
		fmt.Printf("  Injury:      %s  (fatal=%d serious=%d minor=%d uninjured=%d)\n",
			*e.HighestInjury,
			intOrZero(e.TotalFatal), intOrZero(e.TotalSerious),
			intOrZero(e.TotalMinor), intOrZero(e.TotalUninjured))
	}
	if e.PhaseOfFlight != nil {
		fmt.Printf("  Phase:       %s\n", *e.PhaseOfFlight)
	}
	if e.Weather != nil || e.LightCondition != nil {
		fmt.Printf("  Conditions:  %s / %s\n", derefOrEmpty(e.Weather), derefOrEmpty(e.LightCondition))
	}
	fmt.Printf("\nAircraft (%d)\n", len(d.Aircraft))
	for _, a := range d.Aircraft {
		reg := derefOrEmpty(a.Registration)
		if reg == "" {
			reg = "(no registration)"
		}
		typeStr := ""
		if a.Manufacturer != nil && a.Model != nil {
			typeStr = " " + *a.Manufacturer + " " + *a.Model
		}
		yr := ""
		if a.YearMfr != nil {
			yr = fmt.Sprintf(" (%d)", *a.YearMfr)
		}
		fmt.Printf("  #%d  %s%s%s  damage=%s\n", a.AircraftIdx, reg, typeStr, yr, derefOrEmpty(a.Damage))
		if a.OperatorName != nil {
			fmt.Printf("      operator: %s\n", *a.OperatorName)
		}
	}
	if d.Summary != nil {
		fmt.Printf("\nSummary\n  %s\n", *d.Summary)
	}
	if d.Narrative != nil {
		fmt.Printf("\nFull narrative\n%s\n", *d.Narrative)
	}
	return nil
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
