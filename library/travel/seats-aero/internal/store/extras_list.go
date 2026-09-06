// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func (s *Store) TypedTableColumns(table string) ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (s *Store) TypedTableRowCount(ctx context.Context, table string) (int64, error) {
	columns, err := s.TypedTableColumns(table)
	if err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("typed table %q does not exist", table)
	}
	var count int64
	query := `SELECT COUNT(*) FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ListTypedFiltered(ctx context.Context, table string, equality map[string]string, limit, offset int) ([]json.RawMessage, error) {
	columns, err := s.TypedTableColumns(table)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("typed table %q does not exist", table)
	}
	valid := make(map[string]bool, len(columns))
	for _, column := range columns {
		valid[column] = true
	}
	if !valid["data"] {
		return nil, fmt.Errorf("typed table %q has no data column", table)
	}
	keys := make([]string, 0, len(equality))
	for column := range equality {
		if !valid[column] {
			return nil, fmt.Errorf("column %q does not exist in typed table %q", column, table)
		}
		keys = append(keys, column)
	}
	sort.Strings(keys)
	query := `SELECT data FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`
	args := make([]any, 0, len(keys)+2)
	if len(keys) > 0 {
		clauses := make([]string, 0, len(keys))
		for _, column := range keys {
			clauses = append(clauses, `"`+strings.ReplaceAll(column, `"`, `""`)+`" = ?`)
			args = append(args, equality[column])
		}
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY synced_at DESC, id ASC LIMIT ? OFFSET ?"
	if limit <= 0 {
		limit = -1
	}
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func (s *Store) StreamList(ctx context.Context, resourceType string, fn func(json.RawMessage) bool) error {
	columns, err := s.TypedTableColumns("resources")
	if err != nil {
		return err
	}
	orderColumn := "synced_at"
	for _, column := range columns {
		if column == "updated_at" {
			orderColumn = "updated_at"
			break
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = ? ORDER BY "`+orderColumn+`" DESC, id ASC`, resourceType)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return err
		}
		if !fn(json.RawMessage(data)) {
			break
		}
	}
	return rows.Err()
}
