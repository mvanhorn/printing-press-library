// Hand-authored: best-effort actions-ledger logging hook for novel commands.
// Created during Phase 3 build; survives regen because it lives in its own file.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"
)

// logActionBestEffort writes a single Action row to the local SQLite ledger.
//
// It is called from the five leaf commands that drive computers (bash,
// click, screenshot, exec, key) AFTER the API call succeeds. Any failure
// here — DB locked, schema migration pending, file system unavailable —
// must NEVER break the user's command. Returns nil on every error path
// so the caller can `_ = logActionBestEffort(...)` and move on.
//
// snippet may be a JSON document or arbitrary bytes; we store at most the
// first 1024 bytes (the store further caps to 4096 inside LogAction).
// When the snippet is JSON, we re-serialize compactly so the ledger
// preserves structure without unnecessary whitespace.
func logActionBestEffort(ctx context.Context, computerID, kind string, args map[string]any, snippet string, statusCode int, dur time.Duration) error {
	dbPath := defaultDBPath("orgo-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()

	// Compact JSON snippet bodies; leave non-JSON bytes alone.
	if trimmed := strings.TrimSpace(snippet); trimmed != "" && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		var v any
		if json.Unmarshal([]byte(trimmed), &v) == nil {
			if compact, mErr := json.Marshal(v); mErr == nil {
				snippet = string(compact)
			}
		}
	}
	if len(snippet) > 1024 {
		snippet = snippet[:1024]
	}

	// Pull the "command" field out of args when present so the
	// ledger's typed `command` column populates cleanly. The full
	// argument map still goes into args_json so nothing is lost.
	command := ""
	if c, ok := args["command"].(string); ok {
		command = c
	}

	a := store.Action{
		Timestamp:     time.Now().UTC(),
		ComputerID:    computerID,
		Kind:          kind,
		Command:       command,
		ArgsJSON:      store.EncodeActionArgs(args),
		OutputSnippet: snippet,
		StatusCode:    sql.NullInt64{Int64: int64(statusCode), Valid: statusCode != 0},
		DurationMS:    sql.NullInt64{Int64: dur.Milliseconds(), Valid: dur > 0},
	}
	_ = db.LogAction(ctx, a)
	return nil
}
