// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// repairKlaviyoTypedTables runs under migrate's BEGIN IMMEDIATE lock, even when
// user_version is current: reprints can retire required parent columns without
// changing the framework's schema version. Only quoted typed-table DDL is used;
// resources, sync_state and tenant metadata are outside this repair.
func repairKlaviyoTypedTables(ctx context.Context, conn *sql.Conn, migrations []string) error {
	rows, err := conn.QueryContext(ctx, `SELECT m.name, p.name
		FROM sqlite_master AS m, pragma_table_xinfo(m.name) AS p
		WHERE m.type='table' AND p."notnull"=1 AND p.dflt_value IS NULL AND p.hidden=0`)
	if err != nil {
		return fmt.Errorf("reading required columns: %w", err)
	}
	required := map[string][]string{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			rows.Close()
			return err
		}
		required[strings.ToLower(table)] = append(required[strings.ToLower(table)], column)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, ddl := range migrations {
		rest, ok := strings.CutPrefix(ddl, `CREATE TABLE IF NOT EXISTS "`)
		if !ok {
			continue
		}
		table, definition, ok := strings.Cut(rest, `" (`)
		if !ok || len(required[strings.ToLower(table)]) == 0 {
			continue
		}
		current := map[string]bool{}
		// shortcut: generated typed DDL declares quoted columns on separate lines; use SQLite introspection if that format changes.
		for _, line := range strings.Split(definition, "\n") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), `"`); ok {
				if column, _, ok := strings.Cut(rest, `"`); ok {
					current[strings.ToLower(column)] = true
				}
			}
		}
		if len(current) == 0 {
			return fmt.Errorf("no current columns found for typed table %s", table)
		}
		for _, column := range required[strings.ToLower(table)] {
			if !current[strings.ToLower(column)] {
				if err := rebuildKlaviyoTypedTable(ctx, conn, table, definition); err != nil {
					return fmt.Errorf("rebuilding typed table %s: %w", table, err)
				}
				break
			}
		}
	}
	return nil
}

func rebuildKlaviyoTypedTable(ctx context.Context, conn *sql.Conn, table, definition string) error {
	quote := func(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
	replacement := "__pp_schema_repair_" + table
	if _, err := conn.ExecContext(ctx, `CREATE TABLE `+quote(replacement)+` (`+definition); err != nil {
		return fmt.Errorf("creating replacement: %w", err)
	}
	rows, err := conn.QueryContext(ctx, `SELECT n.name
		FROM pragma_table_xinfo(?) AS n JOIN pragma_table_xinfo(?) AS o ON n.name=o.name COLLATE NOCASE
		WHERE n.hidden=0 ORDER BY n.cid`, replacement, table)
	if err != nil {
		return fmt.Errorf("reading shared columns: %w", err)
	}
	// Keep rowids so existing external-content FTS entries retain their identity.
	columns := []string{"rowid"}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			rows.Close()
			return err
		}
		columns = append(columns, quote(column))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	shared := strings.Join(columns, ", ")
	if _, err := conn.ExecContext(ctx, `INSERT INTO `+quote(replacement)+` (`+shared+`) SELECT `+shared+` FROM `+quote(table)); err != nil {
		return fmt.Errorf("copying shared columns: %w", err)
	}
	// DROP removes the old indexes/triggers, including references to retired
	// columns. The normal migration loop recreates the current indexes/triggers.
	if _, err := conn.ExecContext(ctx, `DROP TABLE `+quote(table)); err != nil {
		return fmt.Errorf("dropping old table: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `ALTER TABLE `+quote(replacement)+` RENAME TO `+quote(table)); err != nil {
		return fmt.Errorf("renaming replacement: %w", err)
	}
	return nil
}
