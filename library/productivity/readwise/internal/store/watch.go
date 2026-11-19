// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored: store methods for the watch + webhook subsystem.
// Three tables: watch_saved_search (queries to monitor), watch_seen
// (items already delivered per watch), webhook (delivery targets).
// DDL lives in extras.go migrateExtras — not here.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Watch is a saved query for monitoring new documents or highlights.
type Watch struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	FiltersJSON string `json:"filters_json"`
	CreatedAt   string `json:"created_at"`
}

// Webhook is a delivery target for watch alerts.
type Webhook struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	Secret      string `json:"-"` // never serialize the signing secret
	Events      string `json:"events"`
	IncludeText bool   `json:"include_text"`
	Signed      bool   `json:"signed"`
	CreatedAt   string `json:"created_at"`
}

// DocItem is a minimal document record for watch payloads.
type DocItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	URL     string `json:"url"`
	Summary string `json:"summary,omitempty"`
}

// HLItem is a minimal highlight record for watch payloads.
type HLItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Text   string `json:"text,omitempty"`
}

func nowWatchRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// --- Watch saved searches ---

// AddWatch inserts a new saved watch and returns its id.
func (s *Store) AddWatch(ctx context.Context, name, kind, filtersJSON string) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO watch_saved_search(name, kind, filters_json, created_at) VALUES(?, ?, ?, ?)`,
		name, kind, filtersJSON, nowWatchRFC3339())
	if err != nil {
		return 0, fmt.Errorf("add watch: %w", err)
	}
	return res.LastInsertId()
}

// ListWatches returns all saved watches.
func (s *Store) ListWatches(ctx context.Context) ([]Watch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kind, filters_json, created_at FROM watch_saved_search ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list watches: %w", err)
	}
	defer rows.Close()
	out := make([]Watch, 0)
	for rows.Next() {
		var w Watch
		var name, kind, filtersJSON, created sql.NullString
		if err := rows.Scan(&w.ID, &name, &kind, &filtersJSON, &created); err != nil {
			continue
		}
		w.Name, w.Kind, w.FiltersJSON, w.CreatedAt = name.String, kind.String, filtersJSON.String, created.String
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWatch looks up a watch by id (if ref parses as int) or name.
func (s *Store) GetWatch(ctx context.Context, ref string) (Watch, error) {
	var row *sql.Row
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, name, kind, filters_json, created_at FROM watch_saved_search WHERE id = ?`, id)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, name, kind, filters_json, created_at FROM watch_saved_search WHERE name = ?`, ref)
	}
	var w Watch
	var name, kind, filtersJSON, created sql.NullString
	if err := row.Scan(&w.ID, &name, &kind, &filtersJSON, &created); err != nil {
		if err == sql.ErrNoRows {
			return Watch{}, fmt.Errorf("watch %q not found", ref)
		}
		return Watch{}, fmt.Errorf("get watch: %w", err)
	}
	w.Name, w.Kind, w.FiltersJSON, w.CreatedAt = name.String, kind.String, filtersJSON.String, created.String
	return w, nil
}

// RemoveWatch hard-deletes a watch and its seen-set. Returns rows affected.
func (s *Store) RemoveWatch(ctx context.Context, id int64) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM watch_seen WHERE watch_id = ?`, id); err != nil {
		return 0, fmt.Errorf("remove watch seen: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM watch_saved_search WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("remove watch: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

// --- Seen set ---

// SeenItemIDs returns the set of item ids already delivered for a watch+kind pair.
func (s *Store) SeenItemIDs(ctx context.Context, watchID int64, kind string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT item_id FROM watch_seen WHERE watch_id = ? AND item_kind = ?`, watchID, kind)
	if err != nil {
		return nil, fmt.Errorf("seen item ids: %w", err)
	}
	defer rows.Close()
	seen := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			seen[id] = true
		}
	}
	return seen, rows.Err()
}

// MarkSeen records item ids as delivered for a watch+kind (idempotent).
func (s *Store) MarkSeen(ctx context.Context, watchID int64, kind string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := nowWatchRFC3339()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO watch_seen(watch_id, item_kind, item_id, seen_at) VALUES(?, ?, ?, ?)`,
			watchID, kind, id, now); err != nil {
			return fmt.Errorf("mark seen: %w", err)
		}
	}
	return tx.Commit()
}

// --- Webhooks ---

// AddWebhook inserts a delivery target and returns its id.
func (s *Store) AddWebhook(ctx context.Context, url, secret, events string, includeText bool) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if events == "" {
		events = "*"
	}
	includeTextInt := 0
	if includeText {
		includeTextInt = 1
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook(url, secret, events, include_text, created_at) VALUES(?, ?, ?, ?, ?)`,
		url, secret, events, includeTextInt, nowWatchRFC3339())
	if err != nil {
		return 0, fmt.Errorf("add webhook: %w", err)
	}
	return res.LastInsertId()
}

// ListWebhooks returns all configured webhooks.
func (s *Store) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret, events, include_text, created_at FROM webhook ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()
	out := make([]Webhook, 0)
	for rows.Next() {
		var h Webhook
		var secret, events, created sql.NullString
		var includeText sql.NullInt64
		if err := rows.Scan(&h.ID, &h.URL, &secret, &events, &includeText, &created); err != nil {
			continue
		}
		h.Secret, h.Events, h.CreatedAt = secret.String, events.String, created.String
		h.IncludeText = includeText.Int64 != 0
		h.Signed = h.Secret != ""
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetWebhook returns a webhook by id.
func (s *Store) GetWebhook(ctx context.Context, id int64) (Webhook, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, url, secret, events, include_text, created_at FROM webhook WHERE id = ?`, id)
	var h Webhook
	var secret, events, created sql.NullString
	var includeText sql.NullInt64
	if err := row.Scan(&h.ID, &h.URL, &secret, &events, &includeText, &created); err != nil {
		if err == sql.ErrNoRows {
			return Webhook{}, fmt.Errorf("webhook #%d not found", id)
		}
		return Webhook{}, fmt.Errorf("get webhook: %w", err)
	}
	h.Secret, h.Events, h.CreatedAt = secret.String, events.String, created.String
	h.IncludeText = includeText.Int64 != 0
	h.Signed = h.Secret != ""
	return h, nil
}

// RemoveWebhook hard-deletes a webhook by id; returns rows affected.
func (s *Store) RemoveWebhook(ctx context.Context, id int64) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhook WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("remove webhook: %w", err)
	}
	return res.RowsAffected()
}

// --- Query execution ---

// validDocLocations are the allowed values for the --location filter.
var validDocLocations = map[string]bool{
	"new": true, "later": true, "shortlist": true, "archive": true, "feed": true,
}

// ListDocItems queries documents matching optional location/category/tag filters.
// Uses json_each(documents.data,'$.tags') for tag membership — do NOT join reader_tags.
func (s *Store) ListDocItems(ctx context.Context, locations, categories, tags []string) ([]DocItem, error) {
	// Validate location tokens
	for _, loc := range locations {
		if !validDocLocations[strings.ToLower(loc)] {
			return nil, fmt.Errorf("invalid location %q: must be one of new, later, shortlist, archive, feed", loc)
		}
	}

	var sb strings.Builder
	var args []any
	sb.WriteString(`SELECT id, title, author, url, summary FROM documents WHERE COALESCE(parent_id,'')=''`)

	if len(locations) > 0 {
		placeholders := make([]string, len(locations))
		for i, loc := range locations {
			placeholders[i] = "?"
			args = append(args, loc)
		}
		sb.WriteString(` AND location IN(` + strings.Join(placeholders, ",") + `)`)
	}
	if len(categories) > 0 {
		placeholders := make([]string, len(categories))
		for i, cat := range categories {
			placeholders[i] = "?"
			args = append(args, cat)
		}
		sb.WriteString(` AND category IN(` + strings.Join(placeholders, ",") + `)`)
	}
	if len(tags) > 0 {
		// Use json_each on data JSON tags object — do NOT join reader_tags
		placeholders := make([]string, len(tags))
		for i, tag := range tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		sb.WriteString(` AND EXISTS(SELECT 1 FROM json_each(data,'$.tags') je WHERE je.key IN(` + strings.Join(placeholders, ",") + `))`)
	}

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list doc items: %w", err)
	}
	defer rows.Close()

	var out []DocItem
	for rows.Next() {
		var d DocItem
		var title, author, url, summary sql.NullString
		if err := rows.Scan(&d.ID, &title, &author, &url, &summary); err != nil {
			continue
		}
		d.Title, d.Author, d.URL, d.Summary = title.String, author.String, url.String, summary.String
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListHighlightItems queries highlights matching an FTS query via highlights_fts.
// When includeText is false, t.text is omitted from the SELECT (” placeholder) to
// avoid fetching potentially large text columns when the caller doesn't need them.
// The books JOIN uses CAST(t.book_id AS TEXT) to bridge the INTEGER/TEXT type mismatch
// between highlights.book_id (INTEGER) and books.id (TEXT).
func (s *Store) ListHighlightItems(ctx context.Context, ftsQuery string, includeText bool, watchIDs ...int64) ([]HLItem, error) {
	if ftsQuery == "" {
		return nil, fmt.Errorf("highlight watch requires --query")
	}
	matchQuery := FTSMatchQuery(ftsQuery)
	if matchQuery == "" {
		return nil, fmt.Errorf("invalid highlight query")
	}
	textCol := "''"
	if includeText {
		textCol = "t.text"
	}
	query := `SELECT t.id, COALESCE(b.title,''), COALESCE(b.author,''), COALESCE(t.url,''), ` + textCol + `
		 FROM highlights t
		 LEFT JOIN books b ON b.id = CAST(t.book_id AS TEXT)
		 JOIN highlights_fts ON highlights_fts.rowid = t.rowid
		 WHERE highlights_fts MATCH ?
		 `
	args := []any{matchQuery}
	if len(watchIDs) > 0 {
		query += ` AND NOT EXISTS (SELECT 1 FROM watch_seen ws WHERE ws.watch_id=? AND ws.item_kind='highlights' AND ws.item_id=CAST(t.id AS TEXT))`
		args = append(args, watchIDs[0])
	}
	query += ` ORDER BY rank, t.id LIMIT 200`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list highlight items: %w", err)
	}
	defer rows.Close()

	var out []HLItem
	for rows.Next() {
		var h HLItem
		var title, author, url, text sql.NullString
		if err := rows.Scan(&h.ID, &title, &author, &url, &text); err != nil {
			continue
		}
		h.Title, h.Author, h.URL, h.Text = title.String, author.String, url.String, text.String
		out = append(out, h)
	}
	return out, rows.Err()
}
