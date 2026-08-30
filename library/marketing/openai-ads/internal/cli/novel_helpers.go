// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel support helpers for the Phase 3 commands (regen-safe).
// pp:data-source auto

package cli

import (
	"database/sql"
	"encoding/json"
)

// closeRows finishes with a result set, reporting the first error from either
// iteration or close. database/sql surfaces a mid-iteration failure through
// rows.Err(), not rows.Close(), so checking Close alone lets a truncated scan
// pass as a complete result set — which would make these commands report
// confident answers (orphans, drift, review history) over partial data.
func closeRows(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}

// rowsToMaps converts a slice of structs into []map[string]any via a JSON
// round-trip so any command's result rows can be fed to printAutoTable for
// human table rendering without hand-rolling a mapper per command.
func rowsToMaps(v any) ([]map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var maps []map[string]any
	if err := json.Unmarshal(raw, &maps); err != nil {
		return nil, err
	}
	if maps == nil {
		maps = make([]map[string]any, 0)
	}
	return maps, nil
}
