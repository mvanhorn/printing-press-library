// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// newSQLCmd registers `cartesia-pp-cli sql "<SELECT ...>"` — a read-only
// query gateway over the local SQLite store. Refuses any statement that
// is not a SELECT (anchored to the canonical keyword to keep the guard
// auditable; OpenReadOnly enforces the same property at the driver level
// as belt-and-suspenders).
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "sql [select]",
		Short:       "Run a read-only SELECT against the local SQLite store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Run an ad-hoc SELECT against the local SQLite mirror.

Only SELECT statements are accepted; any other verb is rejected before
the driver runs the query. The store is opened in mode=ro as a second
defense.`,
		Example: `  cartesia-pp-cli sql "SELECT id, status FROM calls LIMIT 10" --json
  cartesia-pp-cli sql "SELECT COUNT(*) FROM metric_results"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			// Strip matching outer quotes some shells preserve when the
			// caller passes a quoted argument (e.g. live-check probes).
			query = trimMatchingOuterQuotes(query)
			// PATCH(greptile #560): allow `WITH ... SELECT` CTEs in addition
			// to bare SELECT. The driver-level mode=ro from OpenReadOnly is
			// the actual write protection; this prefix check is a defensive
			// readability gate, not the security boundary.
			head := strings.ToUpper(query)
			isSelect := strings.HasPrefix(head, "SELECT ") || strings.EqualFold(query, "SELECT")
			isCTE := strings.HasPrefix(head, "WITH ")
			if !isSelect && !isCTE {
				return fmt.Errorf("only SELECT (or WITH ... SELECT) statements allowed (got: %s)", firstToken(query))
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database (read-only): %w", err)
			}
			defer db.Close()

			if limit > 0 && !strings.Contains(strings.ToUpper(query), "LIMIT") {
				query = query + fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("columns: %w", err)
			}

			results, err := scanRowsAsMaps(rows, cols)
			if err != nil {
				return err
			}

			if flags.csv {
				return writeCSV(cmd, cols, results)
			}
			return flags.printJSON(cmd, results)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/cartesia-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Append a LIMIT clause if the query does not already have one")
	return cmd
}

// firstToken returns up to the first 24 chars of a query for error messages.
func firstToken(q string) string {
	q = strings.TrimSpace(q)
	if len(q) > 24 {
		return q[:24] + "…"
	}
	return q
}

// trimMatchingOuterQuotes removes a single layer of matching outer
// quotes (single or double) from q if both ends are the same quote
// character and the quote does not appear in the middle. This makes
// the SELECT guard tolerant of shells that preserve outer quotes
// (e.g. some test harnesses, the scorecard live-check probe).
func trimMatchingOuterQuotes(q string) string {
	if len(q) < 2 {
		return q
	}
	first := q[0]
	last := q[len(q)-1]
	if first != last {
		return q
	}
	if first != '\'' && first != '"' {
		return q
	}
	inner := q[1 : len(q)-1]
	// If the quote character appears in the inner content, the outer
	// pair is not truly matching — leave the query alone.
	if strings.ContainsRune(inner, rune(first)) {
		return q
	}
	return strings.TrimSpace(inner)
}

// scanRowsAsMaps reads every row of rs into a []map[string]any keyed by
// column name. Byte slices are normalized to strings so JSON encoding
// doesn't base64-encode them — the local store keeps JSON blobs as bytes
// by default but consumers expect readable text.
func scanRowsAsMaps(rs *sql.Rows, cols []string) ([]map[string]any, error) {
	results := make([]map[string]any, 0)
	for rs.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			v := raw[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[c] = v
		}
		results = append(results, row)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return results, nil
}

// writeCSV renders results to CSV using the standard library encoder.
func writeCSV(cmd *cobra.Command, cols []string, results []map[string]any) error {
	w := csv.NewWriter(cmd.OutOrStdout())
	if err := w.Write(cols); err != nil {
		return err
	}
	for _, row := range results {
		rec := make([]string, len(cols))
		for i, c := range cols {
			rec[i] = formatCSVCell(row[c])
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func formatCSVCell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}
