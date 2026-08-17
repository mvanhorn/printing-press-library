// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type RunRecord struct {
	ID               string          `json:"id"`
	GroupID          string          `json:"group_id,omitempty"`
	Command          string          `json:"command"`
	Prompt           string          `json:"prompt"`
	Answer           string          `json:"answer,omitempty"`
	Reasoning        string          `json:"reasoning,omitempty"`
	Model            string          `json:"model,omitempty"`
	Seed             int64           `json:"seed,omitempty"`
	ParamsJSON       json.RawMessage `json:"params_json,omitempty"`
	RawJSON          json.RawMessage `json:"raw_json,omitempty"`
	PromptTokens     int             `json:"prompt_tokens,omitempty"`
	CompletionTokens int             `json:"completion_tokens,omitempty"`
	TotalTokens      int             `json:"total_tokens,omitempty"`
	CostUSD          float64         `json:"cost_usd,omitempty"`
	LatencyMS        int64           `json:"latency_ms,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type VaultEntry struct {
	Token     string    `json:"token"`
	Value     string    `json:"value,omitempty"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditRecord struct {
	ID             string    `json:"id"`
	RunID          string    `json:"run_id,omitempty"`
	Command        string    `json:"command"`
	PayloadSHA256  string    `json:"payload_sha256"`
	ByteCount      int       `json:"byte_count"`
	Model          string    `json:"model,omitempty"`
	GuardModel     string    `json:"guard_model,omitempty"`
	MaskedEntities int       `json:"masked_entities"`
	LeakedPIICount int       `json:"leaked_pii_count"`
	CostUSD        float64   `json:"cost_usd,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type ModelCacheRecord struct {
	ID                    string          `json:"id"`
	ContextWindow         int             `json:"context_window"`
	SupportsTools         bool            `json:"supports_tools"`
	SupportsReasoning     bool            `json:"supports_reasoning"`
	InputPricePerMillion  float64         `json:"input_price_per_million"`
	OutputPricePerMillion float64         `json:"output_price_per_million"`
	RawJSON               json.RawMessage `json:"raw_json,omitempty"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

func NewID(prefix string) string {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		n := atomic.AddUint64(&fallbackIDCounter, 1)
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), n)))
		return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(sum[:8]))
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%x", prefix, time.Now().UnixNano(), entropy)))
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(sum[:8]))
}

var fallbackIDCounter uint64

func ValueHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) SaveRun(ctx context.Context, r RunRecord) error {
	if r.ID == "" {
		r.ID = NewID("run")
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs (
		id, group_id, command, prompt, answer, reasoning, model, seed, params_json, raw_json,
		prompt_tokens, completion_tokens, total_tokens, cost_usd, latency_ms, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, nullEmpty(r.GroupID), r.Command, r.Prompt, r.Answer, r.Reasoning, r.Model, zeroNullInt(r.Seed),
		rawOrNull(r.ParamsJSON), rawOrNull(r.RawJSON), r.PromptTokens, r.CompletionTokens, r.TotalTokens,
		r.CostUSD, r.LatencyMS, r.CreatedAt.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs_fts (id, prompt, answer, reasoning) VALUES (?, ?, ?, ?)`,
		r.ID, r.Prompt, r.Answer, r.Reasoning); err != nil {
		return fmt.Errorf("index run: %w", err)
	}
	return tx.Commit()
}

func (s *Store) GetRun(ctx context.Context, id string) (RunRecord, error) {
	var r RunRecord
	var group, answer, reasoning, model sql.NullString
	var seed sql.NullInt64
	var params, raw sql.NullString
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT id, group_id, command, prompt, answer, reasoning, model, seed,
		params_json, raw_json, prompt_tokens, completion_tokens, total_tokens, cost_usd, latency_ms, created_at
		FROM runs WHERE id = ?`, id).Scan(&r.ID, &group, &r.Command, &r.Prompt, &answer, &reasoning, &model, &seed,
		&params, &raw, &r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.CostUSD, &r.LatencyMS, &created)
	if err != nil {
		return r, err
	}
	r.GroupID = group.String
	r.Answer = answer.String
	r.Reasoning = reasoning.String
	r.Model = model.String
	r.Seed = seed.Int64
	if params.Valid {
		r.ParamsJSON = json.RawMessage(params.String)
	}
	if raw.Valid {
		r.RawJSON = json.RawMessage(raw.String)
	}
	r.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return r, nil
}

func (s *Store) SearchRuns(ctx context.Context, query string, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.id FROM runs_fts f JOIN runs r ON r.id = f.id
		WHERE runs_fts MATCH ? ORDER BY rank LIMIT ?`, safeFTSQuery(query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunRecord
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		r, err := s.GetRun(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) RecentRuns(ctx context.Context, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM runs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunRecord
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		r, err := s.GetRun(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) TokenForValue(ctx context.Context, kind, value string) (VaultEntry, bool, error) {
	hash := ValueHash(value)
	var e VaultEntry
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT token, value, kind, created_at FROM vault WHERE value_hash = ?`, hash).
		Scan(&e.Token, &e.Value, &e.Kind, &created)
	if err == sql.ErrNoRows {
		return e, false, nil
	}
	if err != nil {
		return e, false, err
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return e, true, nil
}

func (s *Store) EnsureVaultEntry(ctx context.Context, kind, value string) (VaultEntry, error) {
	hash := ValueHash(value)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return VaultEntry{}, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return VaultEntry{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	existing, ok, err := vaultEntryByHash(ctx, conn, hash)
	if err != nil {
		return VaultEntry{}, err
	}
	if ok {
		if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
			return VaultEntry{}, err
		}
		committed = true
		return existing, nil
	}

	token, err := nextVaultToken(ctx, conn, kind)
	if err != nil {
		return VaultEntry{}, err
	}
	entry := VaultEntry{Token: token, Value: value, Kind: kind, CreatedAt: time.Now().UTC()}
	if _, err := conn.ExecContext(ctx, `INSERT INTO vault (token, value, kind, value_hash, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		entry.Token, entry.Value, entry.Kind, hash, entry.CreatedAt.Format(time.RFC3339)); err != nil {
		return VaultEntry{}, err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return VaultEntry{}, err
	}
	committed = true
	return entry, nil
}

func (s *Store) SaveVaultEntry(ctx context.Context, e VaultEntry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO vault (token, value, kind, value_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(value_hash) DO UPDATE SET token = excluded.token, kind = excluded.kind`,
		e.Token, e.Value, e.Kind, ValueHash(e.Value), e.CreatedAt.Format(time.RFC3339))
	return err
}

type vaultQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func vaultEntryByHash(ctx context.Context, q vaultQueryer, hash string) (VaultEntry, bool, error) {
	var e VaultEntry
	var created string
	err := q.QueryRowContext(ctx, `SELECT token, value, kind, created_at FROM vault WHERE value_hash = ?`, hash).
		Scan(&e.Token, &e.Value, &e.Kind, &created)
	if err == sql.ErrNoRows {
		return e, false, nil
	}
	if err != nil {
		return e, false, err
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return e, true, nil
}

func nextVaultToken(ctx context.Context, q vaultQueryer, kind string) (string, error) {
	for attempts := 0; attempts < 10; attempts++ {
		var suffix [4]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", err
		}
		token := fmt.Sprintf("%s_%s", kind, hex.EncodeToString(suffix[:]))
		var existing string
		err := q.QueryRowContext(ctx, `SELECT token FROM vault WHERE token = ?`, token).Scan(&existing)
		if err == sql.ErrNoRows {
			return token, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate unique vault token for %s", kind)
}

func (s *Store) VaultEntries(ctx context.Context, reveal bool) ([]VaultEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT token, value, kind, created_at FROM vault ORDER BY kind, token`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VaultEntry
	for rows.Next() {
		var e VaultEntry
		var created string
		if err := rows.Scan(&e.Token, &e.Value, &e.Kind, &created); err != nil {
			return nil, err
		}
		if !reveal {
			e.Value = ""
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) VaultTokenMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT token, value FROM vault`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var token, value string
		if err := rows.Scan(&token, &value); err != nil {
			return nil, err
		}
		out[token] = value
	}
	return out, rows.Err()
}

func (s *Store) PurgeVault(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM vault`)
	return err
}

func (s *Store) SaveAudit(ctx context.Context, a AuditRecord) error {
	if a.ID == "" {
		a.ID = NewID("audit")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit (
		id, run_id, command, payload_sha256, byte_count, model, guard_model,
		masked_entities, leaked_pii_count, cost_usd, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, nullEmpty(a.RunID), a.Command, a.PayloadSHA256, a.ByteCount, nullEmpty(a.Model), nullEmpty(a.GuardModel),
		a.MaskedEntities, a.LeakedPIICount, a.CostUSD, a.CreatedAt.Format(time.RFC3339))
	return err
}

func (s *Store) AuditRecords(ctx context.Context, id string, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, run_id, command, payload_sha256, byte_count, model, guard_model,
		masked_entities, leaked_pii_count, cost_usd, created_at FROM audit`
	args := []any{}
	if id != "" {
		query += ` WHERE id = ?`
		args = append(args, id)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRecord
	for rows.Next() {
		var a AuditRecord
		var runID, model, guard sql.NullString
		var created string
		if err := rows.Scan(&a.ID, &runID, &a.Command, &a.PayloadSHA256, &a.ByteCount, &model, &guard,
			&a.MaskedEntities, &a.LeakedPIICount, &a.CostUSD, &created); err != nil {
			return nil, err
		}
		a.RunID = runID.String
		a.Model = model.String
		a.GuardModel = guard.String
		a.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SaveLadder(ctx context.Context, id, prompt string, rungs []string, firstConfident, judge string) error {
	if id == "" {
		id = NewID("ladder")
	}
	encoded, _ := json.Marshal(rungs)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO ladders (id, prompt, rungs, first_confident_model, judge_model, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, prompt, string(encoded), nullEmpty(firstConfident), nullEmpty(judge), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) SaveModelCache(ctx context.Context, m ModelCacheRecord) error {
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = time.Now().UTC()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO models_cache (
		id, context_window, supports_tools, supports_reasoning, input_price_per_million,
		output_price_per_million, raw_json, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET context_window = excluded.context_window,
		supports_tools = excluded.supports_tools,
		supports_reasoning = excluded.supports_reasoning,
		input_price_per_million = excluded.input_price_per_million,
		output_price_per_million = excluded.output_price_per_million,
		raw_json = excluded.raw_json,
		updated_at = excluded.updated_at`,
		m.ID, m.ContextWindow, m.SupportsTools, m.SupportsReasoning, m.InputPricePerMillion,
		m.OutputPricePerMillion, rawOrNull(m.RawJSON), m.UpdatedAt.Format(time.RFC3339))
	return err
}

func (s *Store) QueryModels(ctx context.Context, dsl string) ([]ModelCacheRecord, error) {
	records, err := s.AllModels(ctx)
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(dsl))
	out := make([]ModelCacheRecord, 0, len(records))
	for _, r := range records {
		if modelMatchesTerms(r, terms) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) AllModels(ctx context.Context) ([]ModelCacheRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, context_window, supports_tools, supports_reasoning,
		input_price_per_million, output_price_per_million, raw_json, updated_at FROM models_cache ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelCacheRecord
	for rows.Next() {
		var r ModelCacheRecord
		var raw sql.NullString
		var updated string
		if err := rows.Scan(&r.ID, &r.ContextWindow, &r.SupportsTools, &r.SupportsReasoning,
			&r.InputPricePerMillion, &r.OutputPricePerMillion, &raw, &updated); err != nil {
			return nil, err
		}
		if raw.Valid {
			r.RawJSON = json.RawMessage(raw.String)
		}
		r.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		out = append(out, r)
	}
	return out, rows.Err()
}

func modelMatchesTerms(r ModelCacheRecord, terms []string) bool {
	for _, term := range terms {
		switch {
		case term == "tools" && !r.SupportsTools:
			return false
		case term == "reasoning" && !r.SupportsReasoning:
			return false
		case strings.HasPrefix(term, "ctx>="):
			want := parseContextTerm(strings.TrimPrefix(term, "ctx>="))
			if r.ContextWindow < want {
				return false
			}
		case strings.HasPrefix(term, "ctx>"):
			want := parseContextTerm(strings.TrimPrefix(term, "ctx>"))
			if r.ContextWindow <= want {
				return false
			}
		case strings.HasPrefix(term, "ctx="):
			want := parseContextTerm(strings.TrimPrefix(term, "ctx="))
			if r.ContextWindow != want {
				return false
			}
		case term != "tools" && term != "reasoning" && !strings.Contains(strings.ToLower(r.ID), term):
			return false
		}
	}
	return true
}

func parseContextTerm(s string) int {
	s = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "b")
	mult := 1
	if strings.HasSuffix(s, "k") {
		mult = 1000
		s = strings.TrimSuffix(s, "k")
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n * mult
}

func safeFTSQuery(q string) string {
	tokens := ftsQueryTokenRE.FindAllString(q, -1)
	if len(tokens) == 0 {
		return `""`
	}
	return strings.Join(tokens, " ")
}

func nullEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func zeroNullInt(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func rawOrNull(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
