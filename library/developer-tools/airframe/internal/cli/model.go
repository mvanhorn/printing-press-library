// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newModelCmd() *cobra.Command {
	var (
		since int
		until int
		state string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "model <make-and-model>",
		Short: "Aggregate NTSB events by aircraft make/model with optional year/state filters.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModel(cmd.Context(), strings.Join(args, " "), since, until, state, limit)
		},
	}
	cmd.Flags().IntVar(&since, "since", 0, "Lower bound on event year (inclusive). 0 = no lower bound.")
	cmd.Flags().IntVar(&until, "until", 0, "Upper bound on event year (inclusive). 0 = no upper bound.")
	cmd.Flags().StringVar(&state, "state", "", "Filter to a specific NTSB event_state (two-letter code)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum events to return")
	return cmd
}

type ModelSummary struct {
	MakeModels []MakeModelRow    `json:"make_models"`
	Counts     ModelCounts       `json:"counts"`
	Events     []EventSummaryRow `json:"events"`
}

type ModelCounts struct {
	Total       int `json:"total_events"`
	Fatal       int `json:"fatal_events"`
	Serious     int `json:"serious_events"`
	MinorOrNone int `json:"minor_or_none_events"`
}

func runModel(ctx context.Context, raw string, since, until int, state string, limit int) error {
	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	queryText := strings.TrimSpace(raw)
	if queryText == "" {
		return fmt.Errorf("model query must not be empty")
	}
	if limit <= 0 {
		limit = 200
	}

	mms, err := resolveMakeModelCodes(ctx, st.DB(), queryText)
	if err != nil {
		return err
	}
	summary := &ModelSummary{MakeModels: mms, Events: []EventSummaryRow{}}

	if len(mms) == 0 {
		env := Envelope{
			Meta: Meta{
				Source: "local", DBPath: dbPath, SyncedAt: latestSyncedAt(ctx, st),
				Query: map[string]any{"model": queryText, "since": since, "until": until, "state": state},
			},
			Results: summary,
		}
		if flagJSON || flagSelect != "" {
			return emitEnvelope(env)
		}
		fmt.Printf("No make/model matched %q.\n", queryText)
		return nil
	}

	codes := make([]string, 0, len(mms))
	for _, m := range mms {
		codes = append(codes, m.Code)
	}
	events, counts, err := queryEventsByMakeModelCodes(ctx, st.DB(), codes, since, until, state, limit)
	if err != nil {
		return err
	}
	summary.Events = events
	summary.Counts = counts

	env := Envelope{
		Meta: Meta{
			Source: "local", DBPath: dbPath, SyncedAt: latestSyncedAt(ctx, st),
			Query: map[string]any{"model": queryText, "since": since, "until": until, "state": state, "limit": limit},
		},
		Results: summary,
	}
	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderModelText(queryText, summary)
}

func resolveMakeModelCodes(ctx context.Context, db *sql.DB, query string) ([]MakeModelRow, error) {
	pattern := "%" + query + "%"
	rows, err := db.QueryContext(ctx,
		`SELECT code, manufacturer, model, aircraft_type, engine_type, number_engines, number_seats, weight_class
		FROM make_model
		WHERE (manufacturer || ' ' || model) LIKE ? COLLATE NOCASE
		   OR manufacturer LIKE ? COLLATE NOCASE
		   OR model LIKE ? COLLATE NOCASE
		ORDER BY manufacturer, model`,
		pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("resolve model %q: %w", query, err)
	}
	defer rows.Close()
	var out []MakeModelRow
	for rows.Next() {
		var m MakeModelRow
		var ac, et, wc sql.NullString
		var ne, ns sql.NullInt64
		if err := rows.Scan(&m.Code, &m.Manufacturer, &m.Model, &ac, &et, &ne, &ns, &wc); err != nil {
			return nil, err
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
		out = append(out, m)
	}
	return out, rows.Err()
}

// queryEventsByMakeModelCodes joins event_aircraft → aircraft → make_model_code
// for FAA-registered tails involved in NTSB events. Events where the only
// aircraft is non-US (no FAA row, no make_model_code linkage) are missed in
// v1 — that's a known limitation noted in the dossier output.
func queryEventsByMakeModelCodes(ctx context.Context, db *sql.DB, codes []string, since, until int, state string, limit int) ([]EventSummaryRow, ModelCounts, error) {
	placeholders, codeArgs := placeholdersFor(codes)

	whereParts := []string{
		"(ea.make_model_code IN (" + placeholders + ") OR a.make_model_code IN (" + placeholders + "))",
	}
	// Duplicate the code args (one set per IN clause).
	args := make([]any, 0, len(codeArgs)*2+4)
	args = append(args, codeArgs...)
	args = append(args, codeArgs...)
	if since > 0 {
		whereParts = append(whereParts, "substr(e.event_date,1,4) >= ?")
		args = append(args, fmt.Sprintf("%04d", since))
	}
	if until > 0 {
		whereParts = append(whereParts, "substr(e.event_date,1,4) <= ?")
		args = append(args, fmt.Sprintf("%04d", until))
	}
	if state != "" {
		whereParts = append(whereParts, "e.event_state = ?")
		args = append(args, strings.ToUpper(state))
	}

	q := `SELECT e.event_id, e.event_date, e.event_city, e.event_state, e.highest_injury,
		e.total_fatal, ea.damage, ea.operator_name, e.phase_of_flight, n.summary
	FROM events e
	JOIN event_aircraft ea ON ea.event_id = e.event_id
	LEFT JOIN aircraft a ON a.registration = ea.registration
	LEFT JOIN narratives n ON n.event_id = e.event_id
	WHERE ` + strings.Join(whereParts, " AND ") + `
	ORDER BY e.event_date DESC
	LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, ModelCounts{}, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []EventSummaryRow
	counts := ModelCounts{}
	for rows.Next() {
		var r EventSummaryRow
		var city, st, inj, damage, oper, phase, summary sql.NullString
		var fatal sql.NullInt64
		if err := rows.Scan(&r.EventID, &r.EventDate, &city, &st, &inj, &fatal, &damage, &oper, &phase, &summary); err != nil {
			return nil, counts, err
		}
		r.EventCity = nullToPtr(city)
		r.EventState = nullToPtr(st)
		r.HighestInjury = nullToPtr(inj)
		if fatal.Valid {
			v := int(fatal.Int64)
			r.TotalFatal = &v
		}
		r.Damage = nullToPtr(damage)
		r.OperatorName = nullToPtr(oper)
		r.PhaseOfFlight = nullToPtr(phase)
		r.Summary = nullToPtr(summary)
		events = append(events, r)

		counts.Total++
		switch derefOrEmpty(r.HighestInjury) {
		case "FATL":
			counts.Fatal++
		case "SERS":
			counts.Serious++
		default:
			counts.MinorOrNone++
		}
	}
	return events, counts, rows.Err()
}

func placeholdersFor(items []string) (string, []any) {
	parts := make([]string, len(items))
	args := make([]any, len(items))
	for i, it := range items {
		parts[i] = "?"
		args[i] = it
	}
	return strings.Join(parts, ","), args
}

func renderModelText(query string, s *ModelSummary) error {
	fmt.Printf("Models matching %q (%d codes resolved)\n", query, len(s.MakeModels))
	for i, m := range s.MakeModels {
		if i >= 5 {
			fmt.Printf("  …and %d more\n", len(s.MakeModels)-5)
			break
		}
		fmt.Printf("  %s  %s %s\n", m.Code, m.Manufacturer, m.Model)
	}
	fmt.Printf("\nNTSB events: %d total  (fatal=%d, serious=%d, minor-or-none=%d)\n\n",
		s.Counts.Total, s.Counts.Fatal, s.Counts.Serious, s.Counts.MinorOrNone)
	if len(s.Events) == 0 {
		fmt.Println("  (no events matched)")
		return nil
	}
	for _, e := range s.Events {
		injury := ""
		if e.HighestInjury != nil {
			injury = " " + *e.HighestInjury
		}
		state := ""
		if e.EventState != nil {
			state = " " + *e.EventState
		}
		fmt.Printf("  %s  %s%s%s  %s\n", e.EventDate, e.EventID, injury, state, derefOrEmpty(e.Summary))
	}
	return nil
}
