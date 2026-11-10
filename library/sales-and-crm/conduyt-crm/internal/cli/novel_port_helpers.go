// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// PATCH(novel1219-helpers): response-shape and pagination helpers the hand-written novel commands share
// (send-check, imports blame, imports watch, contacts verify-line-type). They lived in the public library
// entry's helpers.go and dialer_coverage.go; the 4.34.2 reprint regenerated those files without them, so
// they are carried here in one hand-authored file a reprint keeps.

// paginationMaxPages is the common hard ceiling for unbounded novel-command pagination.
const paginationMaxPages = 100

type paginationProgressGuard struct {
	pages       int
	seenIDs     map[string]struct{}
	seenCursors map[string]struct{}
}

func newPaginationProgressGuard() *paginationProgressGuard {
	return &paginationProgressGuard{seenIDs: map[string]struct{}{}, seenCursors: map[string]struct{}{}}
}

// observe records one page. It fails when a full page added no new ids (the server repeats a page), when a cursor
// repeats, or when the page cap is reached while more pages remain.
func (g *paginationProgressGuard) observe(ids []string, nextCursor string, more bool) error {
	g.pages++
	added := 0
	identifiable := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		identifiable++
		if _, exists := g.seenIDs[id]; exists {
			continue
		}
		g.seenIDs[id] = struct{}{}
		added++
	}
	if more && identifiable > 0 && added == 0 {
		return fmt.Errorf("pagination made no progress on page %d (zero new ids)", g.pages)
	}
	if nextCursor != "" {
		if _, exists := g.seenCursors[nextCursor]; exists {
			return fmt.Errorf("pagination cursor %q repeated on page %d", nextCursor, g.pages)
		}
		g.seenCursors[nextCursor] = struct{}{}
	}
	if more && g.pages >= paginationMaxPages {
		return fmt.Errorf("pagination reached the %d-page safety cap", paginationMaxPages)
	}
	return nil
}

func objectPaginationIDs(items []map[string]any) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id := strAny(item, "id", "contactId", "contact_id", "uuid"); id != "" {
			ids = append(ids, id)
			continue
		}
		ids = append(ids, "")
	}
	return ids
}

// rejectResponseErrorEnvelope refuses a 2xx body that is really an error envelope, at the root or in nested data.
func rejectResponseErrorEnvelope(raw json.RawMessage) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	// Any non-null error value marks an error envelope, regardless of its JSON
	// type. A non-empty string or object message does the same at the response
	// root and in data envelopes; null and empty-string messages remain valid.
	for depth := 0; envelope != nil; depth++ {
		if value, ok := envelope["error"]; ok && string(bytes.TrimSpace(value)) != "null" {
			return fmt.Errorf("unexpected error envelope")
		}
		if value, ok := envelope["message"]; ok {
			var text string
			if json.Unmarshal(value, &text) == nil && text != "" {
				return fmt.Errorf("unexpected message envelope: %s", text)
			}
			var detail map[string]json.RawMessage
			if json.Unmarshal(value, &detail) == nil && detail != nil {
				return fmt.Errorf("unexpected message envelope")
			}
		}
		data, ok := envelope["data"]
		var nested map[string]json.RawMessage
		if !ok || json.Unmarshal(data, &nested) != nil {
			break
		}
		envelope = nested
	}
	return nil
}

// hoistPaginatedEnvelope turns {"data":{"data":[...],"meta":{...}}} into {"data":[...],"meta":{...}}.
func hoistPaginatedEnvelope(raw json.RawMessage) (json.RawMessage, bool) {
	var outer map[string]json.RawMessage
	if json.Unmarshal(raw, &outer) != nil || len(outer) != 1 {
		return raw, false
	}
	innerRaw, ok := outer["data"]
	if !ok {
		return raw, false
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(innerRaw, &inner) != nil || len(inner) != 2 {
		return raw, false
	}
	data, dataOK := inner["data"]
	meta, metaOK := inner["meta"]
	if !dataOK || !metaOK {
		return raw, false
	}
	var rows []json.RawMessage
	var metadata map[string]json.RawMessage
	if json.Unmarshal(data, &rows) != nil || json.Unmarshal(meta, &metadata) != nil || metadata == nil {
		return raw, false
	}
	hoisted, err := json.Marshal(map[string]json.RawMessage{"data": data, "meta": meta})
	if err != nil {
		return raw, false
	}
	return hoisted, true
}

// contactsSyncState is the contacts row of sync_state, read through the mirror reader so its failures take the same
// "mirror unusable" path as every other local read. PATCH(novel1219-helpers) r5.
type contactsSyncState struct {
	SyncedAt        time.Time // zero when contacts were never synced
	Total           int       // rows stored when the last attempt ended (sync writes db.Count)
	AttemptComplete bool      // false for an interrupted, aborted or in-progress attempt
}

func readContactsSyncState(ctx context.Context, r mirrorReader) (contactsSyncState, error) {
	return readResourceSyncState(ctx, r, "contacts")
}

// readResourceSyncState reads one resource's sync_state row through the mirror reader (r14: automations hours-audit
// uses it too, so its mirror counts only after a finished attempt, as the contacts readers do).
func readResourceSyncState(ctx context.Context, r mirrorReader, resource string) (contactsSyncState, error) {
	var st contactsSyncState
	rows, err := r.Query(ctx, `SELECT last_synced_at, total_count, last_attempt_complete FROM sync_state WHERE resource_type = ?`, resource)
	if err != nil {
		return st, fmt.Errorf("querying %s sync state: %w", resource, err)
	}
	found := false
	for rows.Next() {
		var syncedAt sql.NullTime
		var total, complete sql.NullInt64
		if err := rows.Scan(&syncedAt, &total, &complete); err != nil {
			_ = rows.Close()
			return st, fmt.Errorf("scanning %s sync state: %w", resource, err)
		}
		found = true
		st = contactsSyncState{SyncedAt: syncedAt.Time, Total: int(total.Int64), AttemptComplete: complete.Valid && complete.Int64 == 1}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return contactsSyncState{}, fmt.Errorf("iterating %s sync state: %w", resource, err)
	}
	if err := rows.Close(); err != nil {
		return contactsSyncState{}, fmt.Errorf("closing %s sync state: %w", resource, err)
	}
	if !found {
		return contactsSyncState{}, nil
	}
	return st, nil
}

// mirrorOlderThan decides freshness from the sync state already read (no second sync_state query that could fail
// silently) and prints the same hint as the framework's stale check.
func mirrorOlderThan(cmd *cobra.Command, syncedAt time.Time, maxAge time.Duration) bool {
	return resourceMirrorOlderThan(cmd, "contacts", syncedAt, maxAge)
}

func resourceMirrorOlderThan(cmd *cobra.Command, resource string, syncedAt time.Time, maxAge time.Duration) bool {
	if maxAge <= 0 || syncedAt.IsZero() {
		return false
	}
	age := time.Since(syncedAt)
	if age <= maxAge {
		return false
	}
	if cmd != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: local %s mirror is %s old, older than --max-age=%s. Run 'conduyt-crm-pp-cli sync --resources %s' to refresh.\n", resource, age.Round(time.Second), maxAge, resource)
	}
	return true
}

// ---- PATCH(novel1219-helpers) r4: one local-read path for the mirror-reading novel commands ----

// mirrorRows is the row cursor of a mirror query (satisfied by *sql.Rows).
type mirrorRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

// mirrorReader is the read surface of the local mirror the novel commands use. Production wraps *sql.DB; tests
// swap newMirrorReader to inject a failure at any stage.
type mirrorReader interface {
	Query(ctx context.Context, query string, args ...any) (mirrorRows, error)
	Count(ctx context.Context, query string, args ...any) (int, error)
}

type sqlMirrorReader struct{ db *sql.DB }

func (r sqlMirrorReader) Query(ctx context.Context, query string, args ...any) (mirrorRows, error) {
	return r.db.QueryContext(ctx, query, args...)
}

func (r sqlMirrorReader) Count(ctx context.Context, query string, args ...any) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

var newMirrorReader = func(db *sql.DB) mirrorReader { return sqlMirrorReader{db} }

// mirrorRow is one stored resource: its id and raw JSON.
type mirrorRow struct {
	ID   string
	Data []byte
}

// readMirrorRows runs one (id, data) query to the end. Any failure at query, scan, iteration or close is returned
// as one error, so a caller can treat the whole read as unusable instead of answering from part of it.
func readMirrorRows(ctx context.Context, r mirrorReader, query string, args ...any) ([]mirrorRow, error) {
	rows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying the mirror: %w", err)
	}
	out := make([]mirrorRow, 0)
	for rows.Next() {
		var row mirrorRow
		if err := rows.Scan(&row.ID, &row.Data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning a mirror row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating the mirror: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing the mirror read: %w", err)
	}
	return out, nil
}
