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
	events, err := queryEventsByMakeModelCodes(ctx, st.DB(), codes, since, until, state, limit)
	if err != nil {
		return err
	}
	// PATCH: totals come from a separate COUNT(DISTINCT) pass so the breakdown
	// reflects every matching event in the database, not just the LIMIT window.
	counts, err := countEventsByMakeModelCodes(ctx, st.DB(), codes, since, until, state)
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
//
// PATCH: dedupes via a `matched` subquery that picks MIN(aircraft_idx) per
// event so a multi-aircraft event (e.g. mid-air between two matching tails)
// surfaces exactly once instead of inflating the result set.
func queryEventsByMakeModelCodes(ctx context.Context, db *sql.DB, codes []string, since, until int, state string, limit int) ([]EventSummaryRow, error) {
	matchClause, matchArgs := matchedEventSubquery(codes)
	whereParts := []string{"e.event_id IN (" + matchClause + ")"}

	// PATCH: args MUST be appended in the same left-to-right order their
	// placeholders appear in the SQL below. The pick-subquery in the JOIN
	// comes first (two IN clauses → 2N placeholders), then the match
	// subquery inside the WHERE (another 2N), then the since/until/state
	// filters, then LIMIT. Reversing pick and match args silently bound
	// filter values to code placeholders whenever a filter was active,
	// returning zero results (Greptile P1 on 5e763246).
	args := make([]any, 0, len(codes)*4+4)
	args = appendCodeArgs(args, codes, 2)
	args = append(args, matchArgs...)
	args, whereParts = appendEventFilters(args, whereParts, since, until, state)

	q := `SELECT e.event_id, e.event_date, e.event_city, e.event_state, e.highest_injury,
		e.total_fatal, ea.damage, ea.operator_name, e.phase_of_flight, n.summary
	FROM events e
	JOIN event_aircraft ea ON ea.event_id = e.event_id AND ea.aircraft_idx = (
		SELECT MIN(ea_pick.aircraft_idx)
		FROM event_aircraft ea_pick
		LEFT JOIN aircraft a_pick ON a_pick.registration = ea_pick.registration
		WHERE ea_pick.event_id = e.event_id
		  AND (ea_pick.make_model_code IN (` + placeholdersOnly(codes) + `)
		    OR a_pick.make_model_code IN (` + placeholdersOnly(codes) + `))
	)
	LEFT JOIN narratives n ON n.event_id = e.event_id
	WHERE ` + strings.Join(whereParts, " AND ") + `
	ORDER BY e.event_date DESC
	LIMIT ?`

	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []EventSummaryRow
	for rows.Next() {
		var r EventSummaryRow
		var city, st, inj, damage, oper, phase, summary sql.NullString
		var fatal sql.NullInt64
		if err := rows.Scan(&r.EventID, &r.EventDate, &city, &st, &inj, &fatal, &damage, &oper, &phase, &summary); err != nil {
			return nil, err
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
	}
	return events, rows.Err()
}

// PATCH: countEventsByMakeModelCodes runs a single COUNT(DISTINCT e.event_id)
// pass without a LIMIT so ModelCounts.Total reflects the full population, then
// breaks down Fatal/Serious/MinorOrNone with conditional COUNTs in the same
// query. Without this, totals reported to text/JSON callers were capped at
// the LIMIT window — misleading for popular models like "Cessna 172".
func countEventsByMakeModelCodes(ctx context.Context, db *sql.DB, codes []string, since, until int, state string) (ModelCounts, error) {
	matchClause, matchArgs := matchedEventSubquery(codes)
	whereParts := []string{"e.event_id IN (" + matchClause + ")"}
	args := append([]any{}, matchArgs...)

	args, whereParts = appendEventFilters(args, whereParts, since, until, state)

	q := `SELECT
		COUNT(DISTINCT e.event_id) AS total,
		COUNT(DISTINCT CASE WHEN e.highest_injury = 'FATL' THEN e.event_id END) AS fatal,
		COUNT(DISTINCT CASE WHEN e.highest_injury = 'SERS' THEN e.event_id END) AS serious,
		COUNT(DISTINCT CASE WHEN e.highest_injury IS NULL OR e.highest_injury NOT IN ('FATL','SERS') THEN e.event_id END) AS minor_or_none
	FROM events e
	WHERE ` + strings.Join(whereParts, " AND ")

	var c ModelCounts
	if err := db.QueryRowContext(ctx, q, args...).Scan(&c.Total, &c.Fatal, &c.Serious, &c.MinorOrNone); err != nil {
		return ModelCounts{}, fmt.Errorf("count events: %w", err)
	}
	return c, nil
}

// matchedEventSubquery returns a SQL fragment (and its bound args) that
// resolves the set of event_ids with at least one aircraft matching codes via
// either event_aircraft.make_model_code or the joined aircraft row.
func matchedEventSubquery(codes []string) (string, []any) {
	placeholders, codeArgs := placeholdersFor(codes)
	q := `SELECT ea_match.event_id
		FROM event_aircraft ea_match
		LEFT JOIN aircraft a_match ON a_match.registration = ea_match.registration
		WHERE ea_match.make_model_code IN (` + placeholders + `)
		   OR a_match.make_model_code IN (` + placeholders + `)`
	args := make([]any, 0, len(codeArgs)*2)
	args = append(args, codeArgs...)
	args = append(args, codeArgs...)
	return q, args
}

func appendEventFilters(args []any, whereParts []string, since, until int, state string) ([]any, []string) {
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
	return args, whereParts
}

func placeholdersOnly(items []string) string {
	parts := make([]string, len(items))
	for i := range items {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func appendCodeArgs(args []any, codes []string, times int) []any {
	for n := 0; n < times; n++ {
		for _, c := range codes {
			args = append(args, c)
		}
	}
	return args
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
