// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// RawQuery runs a read-only SELECT/WITH/EXPLAIN against the local store and
// returns the rows as maps. It backs the novel sql/bench/audit/drift commands,
// which query the Mobbin domain tables created in migrateExtras.
func (s *Store) RawQuery(ctx context.Context, sqlText string) ([]map[string]any, error) {
	token := firstSQLToken(sqlText)
	if token != "SELECT" && token != "WITH" && token != "EXPLAIN" {
		return nil, fmt.Errorf("only SELECT, WITH, and EXPLAIN queries are allowed")
	}
	rows, err := s.db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRawRows(rows)
}

func firstSQLToken(sqlText string) string {
	fields := strings.Fields(strings.TrimSpace(sqlText))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func scanRawRows(rows *sql.Rows) ([]map[string]any, error) {
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
		row := map[string]any{}
		for i, col := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
