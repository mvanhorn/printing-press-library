// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// preseed.go is the cold-start path for the learning subsystem. After a
// sync run, multi-outcome event families (Kalshi events with
// mutually_exclusive=true, Polymarket events with negRisk=true) carry
// every entity a user might ask about: USA, Portugal, England, etc.
// The mapping from (parent.title + child.yes_sub_title) -> child
// resource is mechanically derivable without an LLM teach call. The
// preseed driver registers per-CLI scanners that emit these mappings
// as PreseedRow values; Run iterates the registry and writes the rows
// into search_learnings via direct SQL so the package can stay free
// of any internal/store import (preserving the no-cycle layering that
// internal/store imports internal/learn for normalization helpers).
//
// Idempotency is the load-bearing property. A re-run on an unchanged
// corpus must be a no-op: no new rows, no confidence bumps on
// existing preseed rows, no overwrites of higher-confidence user-
// taught rows. The driver enforces this by checking row existence
// before each insert and skipping writes when a row for
// (query_pattern, resource_id, action) already exists.
//
// Why not call internal/store.Store.UpsertLearning: the store package
// imports internal/learn (for the entity-aware NormalizedQuery type
// that the v3->v4 migration writes into search_learnings). A reverse
// import would form a cycle. Direct SQL is the friction-free shape
// here; the insert columns are identical to what UpsertLearning
// writes (confidence=2 floor, source preserved, query_entities
// JSON-encoded), and the existence-check semantics are stronger
// (preseed never bumps an existing row, period).

package learn

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SourcePreseed is the provenance tag preseed writes into
// search_learnings.source.
const SourcePreseed = "preseed"

// LearningActionBoost mirrors the constant in internal/store; declared
// locally so this file doesn't import store and create a cycle. Any
// drift between the two values would surface as zero recall hits in
// integration tests, which would catch the drift loudly.
const learningActionBoost = "boost"

// preseedConfidenceFloor matches the U4 confidence floor in
// internal/store. Preseed inserts always land at 2 so a freshly-
// preseeded row clears the SKILL.md skip threshold ("confidence >= 2
// + entity_match=exact") on its first observation.
const preseedConfidenceFloor = 2

// DefaultPreseedRowCap caps per-Run total rows to keep a runaway
// scanner from filling the table when sync surfaces an unexpectedly
// large family. Overridable via PRESEED_ROW_CAP; 0 disables the cap.
const DefaultPreseedRowCap = 10000

// PreseedRow is the unit a scanner emits. The driver translates each
// row into one INSERT into search_learnings (subject to the existence
// pre-check).
//
//   - QueryPattern is the raw natural-language query the row
//     represents (e.g., "odds USA wins world cup"). The driver
//     normalizes via store.NormalizeQuery semantics (re-implemented
//     locally to avoid the import cycle).
//   - ResourceID + ResourceType address the row in the resources
//     table.
//   - Venue is "kalshi" / "polymarket".
//   - Entities is the entity slice the scanner extracted from the
//     event/child combination. Stored on the row in the
//     query_entities column as a JSON array.
//   - Source defaults to SourcePreseed when empty.
type PreseedRow struct {
	QueryPattern string
	ResourceID   string
	ResourceType string
	Venue        string
	Entities     []string
	Source       string
}

// ScannerFn is the per-CLI scanner contract. Scanners query the
// just-synced corpus from db (a *sql.DB pointing at the
// prediction-goat SQLite store) and return PreseedRow values.
type ScannerFn func(ctx context.Context, db *sql.DB) ([]PreseedRow, error)

var (
	scannerMu       sync.RWMutex
	scannerRegistry = map[string]ScannerFn{}
)

// RegisterScanner adds a scanner to the package-level registry. Safe
// for concurrent registration. Last writer wins on key collision —
// callers should use a distinct resourceType per scanner.
func RegisterScanner(resourceType string, fn ScannerFn) {
	if fn == nil || resourceType == "" {
		return
	}
	scannerMu.Lock()
	defer scannerMu.Unlock()
	scannerRegistry[resourceType] = fn
}

// ResetScannersForTest clears the registry. Test-only helper.
func ResetScannersForTest() {
	scannerMu.Lock()
	defer scannerMu.Unlock()
	scannerRegistry = map[string]ScannerFn{}
}

// RunOpts tunes a Run invocation.
type RunOpts struct {
	// RowCap overrides DefaultPreseedRowCap. Negative means "use
	// env or default"; zero means "no cap."
	RowCap int
}

// Run iterates every registered scanner, collects PreseedRow values,
// and inserts each into search_learnings via direct SQL. Returns the
// total count of rows actually inserted (skipped duplicates and rows
// where a record already exists do not count). Errors from individual
// scanners are aggregated but do not abort the run.
//
// PRESEED_DISABLED (any non-empty value) skips the entire run.
func Run(ctx context.Context, db *sql.DB) (int, error) {
	return RunWith(ctx, db, RunOpts{RowCap: -1})
}

// RunWith is the explicit-options variant of Run. Exposed so tests
// can pin the row cap deterministically.
func RunWith(ctx context.Context, db *sql.DB, opts RunOpts) (int, error) {
	if os.Getenv("PRESEED_DISABLED") != "" {
		return 0, nil
	}
	if db == nil {
		return 0, errors.New("preseed.Run: db is nil")
	}

	rowCap := resolveRowCap(opts.RowCap)

	scannerMu.RLock()
	scanners := make([]ScannerFn, 0, len(scannerRegistry))
	keys := make([]string, 0, len(scannerRegistry))
	for k, fn := range scannerRegistry {
		scanners = append(scanners, fn)
		keys = append(keys, k)
	}
	scannerMu.RUnlock()

	if len(scanners) == 0 {
		return 0, nil
	}

	var collected []PreseedRow
	var scannerErrs []error
	for i, fn := range scanners {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		rows, err := fn(ctx, db)
		if err != nil {
			scannerErrs = append(scannerErrs, fmt.Errorf("preseed scanner %q: %w", keys[i], err))
			continue
		}
		collected = append(collected, rows...)
	}

	collected = dedupRows(collected)

	if rowCap > 0 && len(collected) > rowCap {
		collected = collected[:rowCap]
	}

	inserted := 0
	for _, row := range collected {
		if err := ctx.Err(); err != nil {
			break
		}
		created, err := insertOne(ctx, db, row)
		if err != nil {
			scannerErrs = append(scannerErrs, fmt.Errorf("preseed upsert %s/%s: %w", row.ResourceType, row.ResourceID, err))
			continue
		}
		if created {
			inserted++
		}
	}

	if len(scannerErrs) > 0 {
		return inserted, errors.Join(scannerErrs...)
	}
	return inserted, nil
}

// insertOne is the per-row write path. Skips when a row exists for
// the same (query_pattern, resource_id, action) regardless of source
// — preseed is a pure-insert operation; UpsertLearning's bump path is
// for taught observations only.
func insertOne(ctx context.Context, db *sql.DB, row PreseedRow) (bool, error) {
	if row.ResourceID == "" || row.QueryPattern == "" {
		return false, nil
	}
	source := row.Source
	if source == "" {
		source = SourcePreseed
	}
	pattern := preseedNormalizeQuery(row.QueryPattern)
	if pattern == "" {
		return false, nil
	}
	action := learningActionBoost

	var existingID int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM search_learnings WHERE query_pattern = ? AND resource_id = ? AND action = ?`,
		pattern, row.ResourceID, action,
	).Scan(&existingID)
	if err == nil {
		return false, nil // row already exists; preseed is no-op
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("preseed existence check: %w", err)
	}

	// Materialize entities as a JSON array string. Always materialize
	// (even when empty) so the column carries a stable shape ("[]"
	// instead of NULL) for downstream introspection.
	entities := row.Entities
	if entities == nil {
		entities = []string{}
	}
	entitiesJSON, err := json.Marshal(entities)
	if err != nil {
		return false, fmt.Errorf("preseed marshal entities: %w", err)
	}

	var venue, resourceType any
	if row.Venue != "" {
		venue = row.Venue
	}
	if row.ResourceType != "" {
		resourceType = row.ResourceType
	}

	now := time.Now().UTC()
	_, err = db.ExecContext(ctx,
		`INSERT INTO search_learnings
		 (query_pattern, query_entities, venue, resource_type, resource_id, action, alias_target,
		  source, confidence, created_at, last_observed_at, notes)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, NULL)`,
		pattern, string(entitiesJSON), venue, resourceType, row.ResourceID, action,
		source, preseedConfidenceFloor, now, now,
	)
	if err != nil {
		return false, fmt.Errorf("preseed insert: %w", err)
	}
	return true, nil
}

// dedupRows collapses identical (normalized pattern, resource) pairs
// to a single entry. The existence check in insertOne is the second
// line of defense.
func dedupRows(in []PreseedRow) []PreseedRow {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]PreseedRow, 0, len(in))
	for _, r := range in {
		key := preseedNormalizeQuery(r.QueryPattern) + "|" + r.ResourceID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}

// resolveRowCap turns a RunOpts.RowCap into the effective cap, with
// env-var fallback.
func resolveRowCap(opts int) int {
	if opts >= 0 {
		return opts
	}
	if v := os.Getenv("PRESEED_ROW_CAP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return DefaultPreseedRowCap
}

// preseedQueryStopwords mirrors the queryStopwords map in
// internal/store/learnings.go. Declared locally because internal/store
// imports this package — a reverse import would form a cycle. Any
// divergence between the two sets would land rows in
// search_learnings.query_pattern that don't match the normalization
// the recall path applies; the test
// TestPreseedRun_RerunIsNoOp catches drift symptomatically.
var preseedQueryStopwords = map[string]struct{}{
	"a": {}, "an": {}, "the": {},
	"what": {}, "whats": {},
	"are": {}, "is": {}, "was": {}, "were": {},
	"do":  {}, "does": {}, "did": {},
	"of":  {}, "for": {}, "to": {}, "on": {},
	"in":  {}, "at": {}, "by": {}, "with": {},
	"odds": {},
}

// preseedNormalizeQuery is a local copy of
// internal/store.NormalizeQuery (lowercase + ASCII + stopword filter,
// preserving insertion order and dropping the standalone "s" token).
// Local copy avoids the reverse import and keeps the two sites
// comparable side by side.
func preseedNormalizeQuery(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	b := strings.Builder{}
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	raw := strings.Fields(b.String())
	seen := make(map[string]struct{}, len(raw))
	kept := make([]string, 0, len(raw))
	for _, t := range raw {
		if t == "" || t == "s" {
			continue
		}
		if _, drop := preseedQueryStopwords[t]; drop {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		kept = append(kept, t)
	}
	return strings.Join(kept, " ")
}
