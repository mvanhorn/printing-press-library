// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Shared helpers for hand-authored Pinecone transcendence commands:
// per-index host resolution, live JSON fetch, and custom local SQLite tables.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/store"
)

// resolveIndexHost returns the data-plane base URL (https://{index_host})
// for the named index by calling the control-plane describe-index endpoint,
// then sets cfg.TemplateVars["index_host"] so the returned client resolves
// {index_host} paths. An explicit PINECONE_INDEX_HOST env var wins (it is
// already populated into TemplateVars at Load time); otherwise the host is
// fetched live.
func resolveIndexHost(ctx context.Context, c *client.Client, indexName string) (string, error) {
	cfg, err := config.Load("")
	if err != nil {
		return "", err
	}
	if h := cfg.TemplateVars["index_host"]; h != "" && h != "index_host_placeholder" {
		return "https://" + h, nil
	}
	// Fetch the index's host from the control plane.
	path := "https://api.pinecone.io/indexes/{index_name}"
	path = replacePathParam(path, "index_name", indexName)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return "", fmt.Errorf("resolving host for index %q: %w", indexName, err)
	}
	var idx struct {
		Host string `json:"host"`
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return "", fmt.Errorf("parsing index %q host: %w", indexName, err)
	}
	if idx.Host == "" {
		return "", fmt.Errorf("index %q has no host (not ready?)", indexName)
	}
	cfg.TemplateVars["index_host"] = idx.Host
	return "https://" + idx.Host, nil
}

// dataPlanePath returns the data-plane path with {index_host} replaced by the
// resolved host (either from the env or from a live describe-index call).
func dataPlanePath(ctx context.Context, c *client.Client, indexName, path string) (string, error) {
	base, err := resolveIndexHost(ctx, c, indexName)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path, nil
}

// apiVersionHeaders returns the required version header map.
func apiVersionHeaders() map[string]string {
	return map[string]string{"X-Pinecone-Api-Version": "2026-04"}
}

// parseDurationLoose parses Go durations plus day/week shorthand (90d, 2w).
func parseDurationLoose(s string) (time.Duration, error) {
	return cliutil.ParseDurationLoose(s)
}

// indexDimension returns the dimension of the named index (control plane).
func indexDimension(ctx context.Context, c *client.Client, indexName string) (int64, error) {
	path := "https://api.pinecone.io/indexes/{index_name}"
	path = replacePathParam(path, "index_name", indexName)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return 0, fmt.Errorf("describing index %q: %w", indexName, err)
	}
	var idx struct {
		Dimension int64 `json:"dimension"`
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return 0, fmt.Errorf("parsing index %q: %w", indexName, err)
	}
	return idx.Dimension, nil
}

// ensureModelDimension verifies the embedding model's default dimension
// matches the index dimension so text-query does not burn a request on a
// dimension mismatch. The inference API exposes the model dimension via
// /models/{model_name}.
func ensureModelDimension(ctx context.Context, c *client.Client, model string, dim int64) error {
	if dim <= 0 {
		return nil
	}
	path := "https://api.pinecone.io/models/{model_name}"
	path = replacePathParam(path, "model_name", model)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		// Unknown model — let the embed call surface the real error.
		return nil
	}
	var m struct {
		DefaultDimension int64 `json:"default_dimension"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if m.DefaultDimension > 0 && m.DefaultDimension != dim {
		return fmt.Errorf("model %q produces %d-dim vectors but index requires %d-dim; choose a matching model with --model", model, m.DefaultDimension, dim)
	}
	return nil
}

// modelDimension returns the default dimension of a hosted embedding model.
func modelDimension(ctx context.Context, c *client.Client, model string) (int64, error) {
	path := "https://api.pinecone.io/models/{model_name}"
	path = replacePathParam(path, "model_name", model)
	data, err := c.Get(ctx, path, map[string]string{})
	if err != nil {
		return 0, err
	}
	var m struct {
		DefaultDimension int64 `json:"default_dimension"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return 0, err
	}
	return m.DefaultDimension, nil
}

// --- Custom local SQLite tables for transcendence features ---

// ensureTranscendTables creates the snapshot + prune + coverage tables on
// demand. Called lazily by the local-store novel commands.
func ensureTranscendTables(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS pp_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			index_name TEXT NOT NULL,
			captured_at TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			total_vectors INTEGER NOT NULL DEFAULT 0,
			dimension INTEGER NOT NULL DEFAULT 0,
			metric TEXT NOT NULL DEFAULT '',
			host TEXT NOT NULL DEFAULT '',
			data TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pp_snapshots_index_time
			ON pp_snapshots (index_name, captured_at)`,
		`CREATE TABLE IF NOT EXISTS pp_prune_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			index_name TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			ran_at TEXT NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0,
			ids TEXT NOT NULL DEFAULT '[]'
		)`,
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("creating local tables: %w", err)
		}
	}
	return nil
}

// novelDBPath resolves the default SQLite path for this CLI.
func novelDBPath() string {
	return defaultDBPath("pinecone-pp-cli")
}

// openNovelDB opens (creating if needed) the local store and ensures the
// transcendence tables exist.
func openNovelDB(ctx context.Context) (*store.Store, *sql.DB, error) {
	dbPath := novelDBPath()
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, nil, err
	}
	db := s.DB()
	if err := ensureTranscendTables(ctx, db); err != nil {
		_ = s.Close()
		return nil, nil, err
	}
	return s, db, nil
}

// missingMirrorHint prints the "no local mirror" guidance used by local-store
// commands. Returns true when the DB file does not exist.
func missingMirrorHint(w io.Writer, dbPath string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(w, "no local mirror at %s\nrun: pinecone-pp-cli snapshot <index> to capture index state first\n", dbPath)
		return true
	}
	return false
}

// defaultNovelDB returns the resolved db path (--db or the platform default),
// creating the parent directory if needed.
func defaultNovelDB(dbPath string) (string, error) {
	if dbPath == "" {
		dbPath = novelDBPath()
	}
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", fmt.Errorf("creating data directory: %w", err)
		}
	}
	return dbPath, nil
}
