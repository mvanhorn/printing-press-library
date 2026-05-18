// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"

	"github.com/spf13/cobra"
)

func newTailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tail <N-number>",
		Short: "Full dossier for an N-number: aircraft + make/model + engine + accident history.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTail(cmd.Context(), args[0])
		},
	}
}

// AircraftRow is the JSON shape for one airframe.
type AircraftRow struct {
	Registration   string  `json:"registration"`
	SerialNumber   *string `json:"serial_number,omitempty"`
	MakeModelCode  *string `json:"make_model_code,omitempty"`
	EngineCode     *string `json:"engine_code,omitempty"`
	YearMfr        *int    `json:"year_mfr,omitempty"`
	TypeRegistrant *string `json:"type_registrant,omitempty"`
	TypeAircraft   *string `json:"type_aircraft,omitempty"`
	TypeEngine     *string `json:"type_engine,omitempty"`
	StatusCode     *string `json:"status_code,omitempty"`
	CertIssueDate  *string `json:"cert_issue_date,omitempty"`
	LastActionDate *string `json:"last_action_date,omitempty"`
	AirworthDate   *string `json:"airworthiness_date,omitempty"`
	ExpirationDate *string `json:"expiration_date,omitempty"`
	ModeSCodeHex   *string `json:"mode_s_code_hex,omitempty"`
	OwnerName      *string `json:"owner_name,omitempty"`
	OwnerStreet    *string `json:"owner_street,omitempty"`
	OwnerCity      *string `json:"owner_city,omitempty"`
	OwnerState     *string `json:"owner_state,omitempty"`
	OwnerZip       *string `json:"owner_zip,omitempty"`
	OwnerCountry   *string `json:"owner_country,omitempty"`
}

type MakeModelRow struct {
	Code          string  `json:"code"`
	Manufacturer  string  `json:"manufacturer"`
	Model         string  `json:"model"`
	AircraftType  *string `json:"aircraft_type,omitempty"`
	EngineType    *string `json:"engine_type,omitempty"`
	NumberEngines *int    `json:"number_engines,omitempty"`
	NumberSeats   *int    `json:"number_seats,omitempty"`
	WeightClass   *string `json:"weight_class,omitempty"`
}

type EngineRow struct {
	Code         string  `json:"code"`
	Manufacturer *string `json:"manufacturer,omitempty"`
	Model        *string `json:"model,omitempty"`
	EngineType   *string `json:"engine_type,omitempty"`
	Horsepower   *int    `json:"horsepower,omitempty"`
}

type EventSummaryRow struct {
	EventID       string  `json:"event_id"`
	EventDate     string  `json:"event_date"`
	EventCity     *string `json:"event_city,omitempty"`
	EventState    *string `json:"event_state,omitempty"`
	HighestInjury *string `json:"highest_injury,omitempty"`
	TotalFatal    *int    `json:"total_fatal,omitempty"`
	Damage        *string `json:"damage,omitempty"`
	OperatorName  *string `json:"operator_name,omitempty"`
	PhaseOfFlight *string `json:"phase_of_flight,omitempty"`
	Summary       *string `json:"summary,omitempty"`
}

type TailDossier struct {
	Aircraft  *AircraftRow      `json:"aircraft,omitempty"`
	MakeModel *MakeModelRow     `json:"make_model,omitempty"`
	Engine    *EngineRow        `json:"engine,omitempty"`
	History   []EventSummaryRow `json:"history"`
}

func runTail(ctx context.Context, raw string) error {
	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	registration := strings.ToUpper(strings.TrimSpace(raw))
	if !strings.HasPrefix(registration, "N") {
		registration = "N" + registration
	}

	dossier := &TailDossier{History: []EventSummaryRow{}}

	dossier.Aircraft, err = queryAircraft(ctx, st.DB(), registration)
	if err != nil {
		return err
	}
	if dossier.Aircraft == nil {
		return fmt.Errorf("no aircraft found with registration %s — try `airframe-pp-cli sync` if your store is stale", registration)
	}
	if dossier.Aircraft.MakeModelCode != nil {
		dossier.MakeModel, _ = queryMakeModel(ctx, st.DB(), *dossier.Aircraft.MakeModelCode)
	}
	if dossier.Aircraft.EngineCode != nil {
		dossier.Engine, _ = queryEngine(ctx, st.DB(), *dossier.Aircraft.EngineCode)
	}
	dossier.History, err = queryHistoryByRegistration(ctx, st.DB(), registration)
	if err != nil {
		return err
	}

	env := Envelope{
		Meta: Meta{
			Source:   "local",
			DBPath:   dbPath,
			SyncedAt: latestSyncedAt(ctx, st),
			Query:    map[string]any{"registration": registration},
		},
		Results: dossier,
	}

	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderTailText(env, dossier)
}

func renderTailText(env Envelope, d *TailDossier) error {
	_ = env
	fmt.Printf("Aircraft %s\n", d.Aircraft.Registration)
	if d.MakeModel != nil {
		fmt.Printf("  Type:    %s %s\n", d.MakeModel.Manufacturer, d.MakeModel.Model)
	}
	if d.Aircraft.YearMfr != nil {
		fmt.Printf("  Year:    %d\n", *d.Aircraft.YearMfr)
	}
	if d.Aircraft.StatusCode != nil {
		fmt.Printf("  Status:  %s\n", *d.Aircraft.StatusCode)
	}
	if d.Aircraft.OwnerName != nil {
		fmt.Printf("  Owner:   %s", *d.Aircraft.OwnerName)
		loc := ""
		if d.Aircraft.OwnerCity != nil {
			loc += *d.Aircraft.OwnerCity
		}
		if d.Aircraft.OwnerState != nil {
			loc += ", " + *d.Aircraft.OwnerState
		}
		if loc != "" {
			fmt.Printf(" (%s)", loc)
		}
		fmt.Println()
	}
	if d.Aircraft.ModeSCodeHex != nil {
		fmt.Printf("  Mode-S:  %s\n", *d.Aircraft.ModeSCodeHex)
	}
	if d.Engine != nil {
		fmt.Printf("  Engine:  ")
		if d.Engine.Manufacturer != nil {
			fmt.Printf("%s ", *d.Engine.Manufacturer)
		}
		if d.Engine.Model != nil {
			fmt.Printf("%s", *d.Engine.Model)
		}
		fmt.Println()
	}

	fmt.Printf("\nHistory (%d NTSB events)\n", len(d.History))
	if len(d.History) == 0 {
		fmt.Println("  (no NTSB-investigated events found)")
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

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func queryAircraft(ctx context.Context, db *sql.DB, registration string) (*AircraftRow, error) {
	const q = `SELECT registration, serial_number, make_model_code, engine_code, year_mfr,
		type_registrant, type_aircraft, type_engine, status_code,
		cert_issue_date, last_action_date, airworthiness_date, expiration_date,
		mode_s_code_hex, owner_name, owner_street, owner_city, owner_state,
		owner_zip, owner_country
	FROM aircraft WHERE registration = ?`
	r := db.QueryRowContext(ctx, q, registration)
	var a AircraftRow
	var serial, mmcode, ecode, treg, tac, ten, sc, cid, lad, awd, exp, ms, name, st, ci, stt, zip, cnt sql.NullString
	var year sql.NullInt64
	if err := r.Scan(&a.Registration, &serial, &mmcode, &ecode, &year,
		&treg, &tac, &ten, &sc, &cid, &lad, &awd, &exp,
		&ms, &name, &st, &ci, &stt, &zip, &cnt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query aircraft %s: %w", registration, err)
	}
	a.SerialNumber = nullToPtr(serial)
	a.MakeModelCode = nullToPtr(mmcode)
	a.EngineCode = nullToPtr(ecode)
	if year.Valid {
		v := int(year.Int64)
		a.YearMfr = &v
	}
	a.TypeRegistrant = nullToPtr(treg)
	a.TypeAircraft = nullToPtr(tac)
	a.TypeEngine = nullToPtr(ten)
	a.StatusCode = nullToPtr(sc)
	a.CertIssueDate = nullToPtr(cid)
	a.LastActionDate = nullToPtr(lad)
	a.AirworthDate = nullToPtr(awd)
	a.ExpirationDate = nullToPtr(exp)
	a.ModeSCodeHex = nullToPtr(ms)
	a.OwnerName = nullToPtr(name)
	a.OwnerStreet = nullToPtr(st)
	a.OwnerCity = nullToPtr(ci)
	a.OwnerState = nullToPtr(stt)
	a.OwnerZip = nullToPtr(zip)
	a.OwnerCountry = nullToPtr(cnt)
	return &a, nil
}

func queryMakeModel(ctx context.Context, db *sql.DB, code string) (*MakeModelRow, error) {
	r := db.QueryRowContext(ctx, `SELECT code, manufacturer, model, aircraft_type, engine_type,
		number_engines, number_seats, weight_class FROM make_model WHERE code = ?`, code)
	var m MakeModelRow
	var ac, et, wc sql.NullString
	var ne, ns sql.NullInt64
	if err := r.Scan(&m.Code, &m.Manufacturer, &m.Model, &ac, &et, &ne, &ns, &wc); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query make_model %s: %w", code, err)
	}
	m.AircraftType = nullToPtr(ac)
	m.EngineType = nullToPtr(et)
	if ne.Valid {
		v := int(ne.Int64)
		m.NumberEngines = &v
	}
	if ns.Valid {
		v := int(ns.Int64)
		m.NumberSeats = &v
	}
	m.WeightClass = nullToPtr(wc)
	return &m, nil
}

func queryEngine(ctx context.Context, db *sql.DB, code string) (*EngineRow, error) {
	r := db.QueryRowContext(ctx, `SELECT code, manufacturer, model, engine_type, horsepower
		FROM engine WHERE code = ?`, code)
	var e EngineRow
	var mfr, model, et sql.NullString
	var hp sql.NullInt64
	if err := r.Scan(&e.Code, &mfr, &model, &et, &hp); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query engine %s: %w", code, err)
	}
	e.Manufacturer = nullToPtr(mfr)
	e.Model = nullToPtr(model)
	e.EngineType = nullToPtr(et)
	if hp.Valid {
		v := int(hp.Int64)
		e.Horsepower = &v
	}
	return &e, nil
}

func queryHistoryByRegistration(ctx context.Context, db *sql.DB, registration string) ([]EventSummaryRow, error) {
	const q = `SELECT e.event_id, e.event_date, e.event_city, e.event_state, e.highest_injury,
		e.total_fatal, ea.damage, ea.operator_name, e.phase_of_flight, n.summary
	FROM events e
	JOIN event_aircraft ea ON ea.event_id = e.event_id
	LEFT JOIN narratives n ON n.event_id = e.event_id
	WHERE ea.registration = ?
	ORDER BY e.event_date DESC`
	rows, err := db.QueryContext(ctx, q, registration)
	if err != nil {
		return nil, fmt.Errorf("query history for %s: %w", registration, err)
	}
	defer rows.Close()
	var out []EventSummaryRow
	for rows.Next() {
		var r EventSummaryRow
		var city, state, inj, damage, oper, phase, summary sql.NullString
		var fatal sql.NullInt64
		if err := rows.Scan(&r.EventID, &r.EventDate, &city, &state, &inj, &fatal, &damage, &oper, &phase, &summary); err != nil {
			return nil, err
		}
		r.EventCity = nullToPtr(city)
		r.EventState = nullToPtr(state)
		r.HighestInjury = nullToPtr(inj)
		if fatal.Valid {
			v := int(fatal.Int64)
			r.TotalFatal = &v
		}
		r.Damage = nullToPtr(damage)
		r.OperatorName = nullToPtr(oper)
		r.PhaseOfFlight = nullToPtr(phase)
		r.Summary = nullToPtr(summary)
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullToPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// openReadStore resolves the configured DB path and opens it read-only.
func openReadStore(_ context.Context) (string, *store.Store, error) {
	dbPath := flagDBPath
	if dbPath == "" {
		dbPath = store.DefaultDBPath()
	}
	st, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return dbPath, nil, fmt.Errorf("opening store at %s: %w", dbPath, err)
	}
	return dbPath, st, nil
}

// latestSyncedAt returns the most recent last_synced_at across all sources
// in sync_meta, or "" if the table is empty.
func latestSyncedAt(ctx context.Context, st *store.Store) string {
	rows, err := st.ListSyncMeta(ctx)
	if err != nil {
		return ""
	}
	latest := ""
	for _, r := range rows {
		if r.LastSyncedAt > latest {
			latest = r.LastSyncedAt
		}
	}
	return latest
}
