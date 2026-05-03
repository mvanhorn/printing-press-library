// Hand-written transcendence helpers. Used by the novel-feature commands
// (links stale/drift/duplicates/lint/rewrite/rollup, funnel, health, since,
// partners leaderboard / audit-commissions, bounties triage / payout-projection,
// customers journey).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/dub/internal/store"
)

// openLocalStore opens the on-disk SQLite store at dbPath (or the default if
// dbPath is empty). Callers are responsible for closing the returned Store.
func openLocalStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("dub-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database (%s): %w (run `dub-pp-cli sync` first)", dbPath, err)
	}
	return db, nil
}

// extractFromData unmarshals a typed table's stored JSON `data` blob and
// returns the value at the dotted path (e.g. "metrics.clicks"). Returns
// "" if the path is missing or the value is non-string.
func extractFromData(blob []byte, path string) string {
	if len(blob) == 0 {
		return ""
	}
	var m any
	if err := json.Unmarshal(blob, &m); err != nil {
		return ""
	}
	parts := strings.Split(path, ".")
	for _, p := range parts {
		obj, ok := m.(map[string]any)
		if !ok {
			return ""
		}
		v, ok := obj[p]
		if !ok {
			return ""
		}
		m = v
	}
	switch v := m.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// extractNumber returns the numeric value at the dotted path or 0 if missing.
func extractNumber(blob []byte, path string) float64 {
	if len(blob) == 0 {
		return 0
	}
	var m any
	if err := json.Unmarshal(blob, &m); err != nil {
		return 0
	}
	parts := strings.Split(path, ".")
	for _, p := range parts {
		obj, ok := m.(map[string]any)
		if !ok {
			return 0
		}
		v, ok := obj[p]
		if !ok {
			return 0
		}
		m = v
	}
	switch v := m.(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

// hintEmptyStore returns a friendly error when a transcendence command opens
// the store and finds zero rows in its primary table.
func hintEmptyStore(resource string) error {
	return fmt.Errorf("local store has no %s rows yet — run `dub-pp-cli sync` first to populate it", resource)
}
