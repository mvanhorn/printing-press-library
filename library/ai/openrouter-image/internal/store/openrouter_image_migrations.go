// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Custom store schema for openrouter-image-pp-cli novel features.
// Extended store schema pattern: runs its own CREATE TABLE IF NOT EXISTS from
// a lazy init invoked by the novel commands that need it. Do not edit the
// migration slice in store.go.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureOpenRouterImageTables creates the custom tables used by the novel
// commands (generation ledger, per-model endpoint pricing cache, and catalog
// snapshot). Safe to call repeatedly; CREATE TABLE IF NOT EXISTS is a no-op.
func (s *Store) EnsureOpenRouterImageTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS generation_ledger (
			id TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			prompt TEXT,
			params TEXT,
			cost_usd REAL,
			tokens TEXT,
			output_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_generation_ledger_created ON generation_ledger(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_generation_ledger_model ON generation_ledger(model)`,
		`CREATE TABLE IF NOT EXISTS image_endpoint_cache (
			model_id TEXT PRIMARY KEY,
			data TEXT NOT NULL,
			fetched_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS image_catalog_snapshot (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			snapshot TEXT NOT NULL,
			taken_at DATETIME NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring openrouter image table: %w", err)
		}
	}
	return nil
}

// GenerationEntry is one row of the local generation ledger.
type GenerationEntry struct {
	ID         string    `json:"id"`
	Model      string    `json:"model"`
	Prompt     string    `json:"prompt"`
	Params     string    `json:"params,omitempty"`
	CostUSD    float64   `json:"cost_usd"`
	Tokens     string    `json:"tokens,omitempty"`
	OutputPath string    `json:"output_path,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// LedgerGeneration records a completed generation in the ledger.
func (s *Store) LedgerGeneration(ctx context.Context, e GenerationEntry) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO generation_ledger (id, model, prompt, params, cost_usd, tokens, output_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP))`,
		e.ID, e.Model, e.Prompt, e.Params, e.CostUSD, e.Tokens, e.OutputPath, e.CreatedAt.Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("inserting generation ledger row: %w", err)
	}
	return nil
}

// GetGeneration returns one ledger row by id.
func (s *Store) GetGeneration(ctx context.Context, id string) (*GenerationEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, model, COALESCE(prompt,''), COALESCE(params,''), COALESCE(cost_usd,0), COALESCE(tokens,''), COALESCE(output_path,''), COALESCE(created_at, CURRENT_TIMESTAMP)
		 FROM generation_ledger WHERE id = ?`, id)
	var e GenerationEntry
	var createdAt string
	if err := row.Scan(&e.ID, &e.Model, &e.Prompt, &e.Params, &e.CostUSD, &e.Tokens, &e.OutputPath, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reading generation %s: %w", id, err)
	}
	if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		e.CreatedAt = t
	} else if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		e.CreatedAt = t
	}
	return &e, nil
}

// ListGenerations returns ledger rows, newest first, optionally filtered to
// the last window.
func (s *Store) ListGenerations(ctx context.Context, since time.Time, limit int) ([]GenerationEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, model, COALESCE(prompt,''), COALESCE(params,''), COALESCE(cost_usd,0), COALESCE(tokens,''), COALESCE(output_path,''), COALESCE(created_at, CURRENT_TIMESTAMP)
		 FROM generation_ledger WHERE created_at >= ? ORDER BY created_at DESC LIMIT ?`,
		since.Format("2006-01-02 15:04:05"), limit)
	if err != nil {
		return nil, fmt.Errorf("listing generations: %w", err)
	}
	defer rows.Close()
	out := make([]GenerationEntry, 0)
	for rows.Next() {
		var e GenerationEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.Model, &e.Prompt, &e.Params, &e.CostUSD, &e.Tokens, &e.OutputPath, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning generation row: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			e.CreatedAt = t
		} else if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			e.CreatedAt = t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EndpointCacheEntry is one model's cached per-endpoint records.
type EndpointCacheEntry struct {
	ModelID   string          `json:"model_id"`
	Data      json.RawMessage `json:"data"`
	FetchedAt time.Time       `json:"fetched_at"`
}

// GetEndpointCache returns a cached endpoint record for a model, or nil.
func (s *Store) GetEndpointCache(ctx context.Context, modelID string) (*EndpointCacheEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT model_id, data, fetched_at FROM image_endpoint_cache WHERE model_id = ?`, modelID)
	var e EndpointCacheEntry
	var fetched string
	var dataStr string
	if err := row.Scan(&e.ModelID, &dataStr, &fetched); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reading endpoint cache %s: %w", modelID, err)
	}
	e.Data = json.RawMessage(dataStr)
	if t, err := time.Parse(time.RFC3339, fetched); err == nil {
		e.FetchedAt = t
	}
	return &e, nil
}

// PutEndpointCache stores a model's endpoint records with a fetch timestamp.
func (s *Store) PutEndpointCache(ctx context.Context, modelID string, data json.RawMessage) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO image_endpoint_cache (model_id, data, fetched_at) VALUES (?, ?, ?)`,
		modelID, string(data), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("caching endpoints for %s: %w", modelID, err)
	}
	return nil
}

// CatalogSnapshot is a stored snapshot of the image model catalog.
type CatalogSnapshot struct {
	Snapshot []map[string]any `json:"snapshot"`
	TakenAt  time.Time        `json:"taken_at"`
}

// GetCatalogSnapshot returns the stored catalog snapshot or nil.
func (s *Store) GetCatalogSnapshot(ctx context.Context) (*CatalogSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `SELECT snapshot, taken_at FROM image_catalog_snapshot WHERE id = 1`)
	var snap string
	var taken string
	if err := row.Scan(&snap, &taken); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reading catalog snapshot: %w", err)
	}
	var out CatalogSnapshot
	if err := json.Unmarshal([]byte(snap), &out.Snapshot); err != nil {
		return nil, fmt.Errorf("parsing catalog snapshot: %w", err)
	}
	if t, err := time.Parse(time.RFC3339, taken); err == nil {
		out.TakenAt = t
	}
	return &out, nil
}

// PutCatalogSnapshot stores the current catalog snapshot.
func (s *Store) PutCatalogSnapshot(ctx context.Context, snapshot []map[string]any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshaling catalog snapshot: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO image_catalog_snapshot (id, snapshot, taken_at) VALUES (1, ?, ?)`,
		string(data), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("storing catalog snapshot: %w", err)
	}
	return nil
}
