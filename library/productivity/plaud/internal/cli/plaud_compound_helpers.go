// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the Plaud transcendence + escape-hatch commands
// (search, sql, commitments, topic, about, forgotten, themes,
// cross-meeting, silence, mentioned-me). Open the local SQLite store,
// run a query, emit rows through printJSONFiltered so --json / --select /
// --csv / --compact / --quiet all work uniformly.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

const plaudCLIName = "plaud-pp-cli"

// openPlaudStore opens the local store at the canonical path. Returns a
// helpful error when the DB doesn't exist yet (user needs to run sync).
func openPlaudStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath(plaudCLIName)
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store at %s: %w (run `plaud-pp-cli sync` first to populate)", dbPath, err)
	}
	return s, nil
}

// parseSinceFlag turns "30d", "12h", or an ISO timestamp into an epoch
// seconds threshold. Returns 0 (meaning "no lower bound") when input is empty.
func parseSinceFlag(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	re := regexp.MustCompile(`^(\d+)([dhm])$`)
	if m := re.FindStringSubmatch(s); m != nil {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		var dur time.Duration
		switch m[2] {
		case "d":
			dur = time.Duration(n) * 24 * time.Hour
		case "h":
			dur = time.Duration(n) * time.Hour
		case "m":
			dur = time.Duration(n) * time.Minute
		}
		return time.Now().Add(-dur).Unix(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid --since value %q (expected '30d', '12h', or ISO timestamp)", s)
}

// queryRowsToMaps runs a SELECT and returns each row as a map[string]any keyed
// by column name. Used by `sql` and several transcendence commands.
func queryRowsToMaps(ctx context.Context, db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
