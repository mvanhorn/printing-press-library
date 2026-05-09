// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"judgementtw-pp-cli/internal/extract"
	"judgementtw-pp-cli/internal/judicial"
	"judgementtw-pp-cli/internal/source/fjud"
	"judgementtw-pp-cli/internal/source/fjudkm"
)

// judicialDBPath returns the SQLite path used by every judicial command.
// Centralised so tests can override it via JUDICIAL_DB env var.
func judicialDBPath() string {
	if env := strings.TrimSpace(getEnv("JUDICIAL_DB")); env != "" {
		return env
	}
	return defaultDBPath("judgementtw-pp-cli")
}

// openJudicialDB opens the local SQLite database and ensures the
// judicial-specific tables exist. Returns a connection the caller closes.
func openJudicialDB(ctx context.Context) (*sql.DB, error) {
	path := judicialDBPath()
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("creating db dir %s: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	if err := judicial.EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureGeneratedSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ensureGeneratedSchema makes sure the generator-emitted tables exist before
// the judicial repos try to JOIN against them. The generator's store.Open
// migrations run when the user invokes `sync`; the novel-feature commands
// create the DB directly via database/sql to avoid pulling in the generator's
// type system.
func ensureGeneratedSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS judgments (
			id TEXT PRIMARY KEY,
			data TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge (
			id TEXT PRIMARY KEY,
			data TEXT,
			updated_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_judgments_id ON judgments(id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensuring generated table: %w", err)
		}
	}
	return nil
}

// fjudClient builds a rate-limited FJUD client. The default rate is 1 req/sec
// to respect the public site; --rate-limit on the root flag overrides.
func fjudClient(flags *rootFlags) *fjud.Client {
	rate := flags.rateLimit
	if rate <= 0 {
		rate = 1.0
	}
	return fjud.New(rate)
}

// fjudkmClient builds a rate-limited FJUDKM client.
func fjudkmClient(flags *rootFlags) *fjudkm.Client {
	rate := flags.rateLimit
	if rate <= 0 {
		rate = 1.0
	}
	return fjudkm.New(rate)
}

// emitJSON writes the value as JSON honouring --select / --compact / --json
// pipeline of the root flags.
func emitJSON(w io.Writer, v any, flags *rootFlags) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(w, raw, flags)
}

// upsertJudgmentRow stores a fjud.Judgment in the generator's `judgments`
// table so cross-judgment queries (cited-by, related) can JOIN against it.
// Also indexes citations and sentences.
func upsertJudgmentRow(ctx context.Context, db *sql.DB, j *fjud.Judgment) error {
	if j == nil {
		return nil
	}
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO judgments (id, data, updated_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		j.JID, string(data))
	if err != nil {
		return err
	}

	citations := extract.ExtractCitations(j.JFullContent)
	jidRefs := extract.ExtractJIDReferences(j.JFullContent)
	if err := judicial.IndexCitations(ctx, db, j.JID, citations, jidRefs); err != nil {
		return err
	}
	sentences := extract.ExtractSentences(j.JFullContent)
	return judicial.IndexSentences(ctx, db, j.JID, sentences)
}

// loadJudgmentRow returns a previously-synced judgment, or (nil, nil) when
// not present.
func loadJudgmentRow(ctx context.Context, db *sql.DB, jid string) (*fjud.Judgment, error) {
	row := db.QueryRowContext(ctx, `SELECT data FROM judgments WHERE id = ?`, jid)
	var blob string
	if err := row.Scan(&blob); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var j fjud.Judgment
	if err := json.Unmarshal([]byte(blob), &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// listJudgmentIDs walks the local store, optionally filtered by court and
// case-type prefix.
func listJudgmentIDs(ctx context.Context, db *sql.DB, court, caseType string, limit int) ([]string, error) {
	q := `SELECT id FROM judgments`
	var args []any
	var conds []string
	if court != "" {
		conds = append(conds, `SUBSTR(id, 1, 3) = ?`)
		args = append(args, court)
	}
	if caseType != "" {
		// CaseType is the 4th character of the court+type prefix.
		conds = append(conds, `SUBSTR(id, 4, 1) = ?`)
		args = append(args, strings.ToUpper(caseType))
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// parseCSVList trims and splits a comma-separated CLI flag value into a list,
// dropping empty entries.
func parseCSVList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveCaseTypes converts a comma-separated CLI value containing either
// long-form names (criminal, civil) or short codes (M, V) into the 1-letter
// codes the FJUD form wants.
func resolveCaseTypes(s string) []string {
	in := parseCSVList(s)
	out := make([]string, 0, len(in))
	for _, t := range in {
		out = append(out, extract.CaseTypeFromEnglish(t))
	}
	return out
}

// getEnv is a tiny os.Getenv wrapper isolated for tests.
func getEnv(k string) string {
	return getEnvImpl(k)
}
