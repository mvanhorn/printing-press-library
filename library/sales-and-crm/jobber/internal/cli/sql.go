// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/internal/store"
	"github.com/spf13/cobra"
)

var sqlBannedTokenRE = regexp.MustCompile(`\b(INSERT|UPDATE|DELETE|REPLACE|DROP|CREATE|ALTER|ATTACH|DETACH|PRAGMA|VACUUM|REINDEX|TRUNCATE|WITH)\b`)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var filePath string
	var explain bool
	var limit int
	var maxCellBytes int

	cmd := &cobra.Command{
		Use:         "sql [query]",
		Short:       "Run read-only ad-hoc SQL against the local synced database",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  jobber-pp-cli sql "SELECT id, invoice_number FROM invoices LIMIT 10"
  jobber-pp-cli sql --file query.sql
  jobber-pp-cli sql --explain "SELECT id FROM resources LIMIT 1"
  jobber-pp-cli sql --json "SELECT id, name FROM clients LIMIT 10" > rows.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			query, err := resolveSQLQuery(filePath, args)
			if err != nil {
				return err
			}
			if err := validateReadOnlySQL(query); err != nil {
				return err
			}
			if explain {
				query = "EXPLAIN QUERY PLAN " + strings.TrimRightFunc(strings.TrimSpace(query), func(r rune) bool {
					return r == ';' || unicode.IsSpace(r)
				})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jobber-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database read-only: %w\nRun 'jobber-pp-cli sync' first.", err)
			}
			defer db.Close()
			if err := db.DB().PingContext(cmd.Context()); err != nil {
				return fmt.Errorf("opening local database read-only: %w\nRun 'jobber-pp-cli sync' first.", err)
			}

			return runSQLQuery(cmd, db, query, limit, maxCellBytes, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("jobber-pp-cli"), "Database path")
	cmd.Flags().StringVar(&filePath, "file", "", "Read query from file instead of args")
	cmd.Flags().BoolVar(&explain, "explain", false, "Run EXPLAIN QUERY PLAN for the query")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Hard row cap after fetch (0 for no cap)")
	cmd.Flags().IntVar(&maxCellBytes, "max-cell-bytes", 4096, "Truncate string/blob cells longer than N bytes (0 for no truncation)")

	return cmd
}

func resolveSQLQuery(filePath string, args []string) (string, error) {
	if filePath != "" && len(args) > 0 {
		return "", fmt.Errorf("use either --file or a positional query, not both")
	}
	if filePath == "" && len(args) != 1 {
		return "", fmt.Errorf("expected exactly one SQL query argument or --file")
	}
	if filePath != "" {
		b, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading SQL file: %w", err)
		}
		return string(b), nil
	}
	return args[0], nil
}

func validateReadOnlySQL(query string) error {
	normalized := normalizeSQLForValidation(query)
	if token := sqlBannedTokenRE.FindString(normalized); token != "" {
		return fmt.Errorf("query rejected: read-only SQL does not allow token %q", token)
	}
	if hasMultipleSQLStatements(query) {
		return fmt.Errorf("query rejected: only one SQL statement is allowed")
	}
	return nil
}

func normalizeSQLForValidation(query string) string {
	var b strings.Builder
	b.Grow(len(query))
	inLineComment := false
	inBlockComment := false
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}

		switch {
		case inLineComment:
			if ch == '\n' || ch == '\r' {
				inLineComment = false
				b.WriteByte(' ')
			}
		case inBlockComment:
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
		case inSingleQuote:
			if ch == '\'' {
				if next == '\'' {
					i++
				} else {
					inSingleQuote = false
				}
			}
			b.WriteByte(' ')
		case inDoubleQuote:
			if ch == '"' {
				if next == '"' {
					i++
				} else {
					inDoubleQuote = false
				}
			}
			b.WriteByte(' ')
		case ch == '-' && next == '-':
			inLineComment = true
			i++
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
		case ch == '\'':
			inSingleQuote = true
			b.WriteByte(' ')
		case ch == '"':
			inDoubleQuote = true
			b.WriteByte(' ')
		default:
			b.WriteRune(unicode.ToUpper(rune(ch)))
		}
	}
	return b.String()
}

func hasMultipleSQLStatements(query string) bool {
	cleaned := normalizeSQLForStatementCheck(query)
	idx := strings.IndexByte(cleaned, ';')
	if idx < 0 {
		return false
	}
	return strings.TrimSpace(cleaned[idx+1:]) != ""
}

func normalizeSQLForStatementCheck(query string) string {
	var b strings.Builder
	b.Grow(len(query))
	inLineComment := false
	inBlockComment := false
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}

		switch {
		case inLineComment:
			if ch == '\n' || ch == '\r' {
				inLineComment = false
				b.WriteByte(ch)
			}
		case inBlockComment:
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
		case inSingleQuote:
			if ch == '\'' {
				if next == '\'' {
					i++
				} else {
					inSingleQuote = false
				}
			}
			b.WriteByte(' ')
		case inDoubleQuote:
			if ch == '"' {
				if next == '"' {
					i++
				} else {
					inDoubleQuote = false
				}
			}
			b.WriteByte(' ')
		case ch == '-' && next == '-':
			inLineComment = true
			i++
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
		case ch == '\'':
			inSingleQuote = true
			b.WriteByte(' ')
		case ch == '"':
			inDoubleQuote = true
			b.WriteByte(' ')
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

func runSQLQuery(cmd *cobra.Command, db *store.Store, query string, limit, maxCellBytes int, flags *rootFlags) error {
	rows, err := db.DB().QueryContext(cmd.Context(), query)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	if flags.asJSON {
		return writeSQLJSON(rows, cols, limit, maxCellBytes)
	}
	return writeSQLTable(cmd, rows, cols, limit, maxCellBytes)
}

func writeSQLJSON(rows *sql.Rows, cols []string, limit, maxCellBytes int) error {
	enc := json.NewEncoder(os.Stdout)
	rowCount := 0
	limitHit := false
	truncatedAny := false

	for rows.Next() {
		if limit > 0 && rowCount >= limit {
			limitHit = true
			break
		}
		values, truncated, err := scanSQLRow(rows, len(cols), maxCellBytes)
		if err != nil {
			return err
		}
		truncatedAny = truncatedAny || truncated
		obj := make(map[string]any, len(cols))
		for i, col := range cols {
			obj[col] = values[i]
		}
		if err := enc.Encode(obj); err != nil {
			return err
		}
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return enc.Encode(map[string]any{
		"event":     "sql_summary",
		"row_count": rowCount,
		"truncated": truncatedAny,
		"limit_hit": limitHit,
	})
}

func writeSQLTable(cmd *cobra.Command, rows *sql.Rows, cols []string, limit, maxCellBytes int) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(cols, "\t"))

	rowCount := 0
	limitHit := false
	truncatedAny := false
	for rows.Next() {
		if limit > 0 && rowCount >= limit {
			limitHit = true
			break
		}
		values, truncated, err := scanSQLRow(rows, len(cols), maxCellBytes)
		if err != nil {
			return err
		}
		truncatedAny = truncatedAny || truncated
		cells := make([]string, len(values))
		for i, value := range values {
			cells[i] = fmt.Sprint(value)
			if value == nil {
				cells[i] = "NULL"
			}
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Fprintf(tw, "-- %d rows (limit hit: %t, truncated cells: %t)\n", rowCount, limitHit, truncatedAny)
	return tw.Flush()
}

func scanSQLRow(rows *sql.Rows, colCount int, maxCellBytes int) ([]any, bool, error) {
	values := make([]any, colCount)
	dest := make([]any, colCount)
	for i := range values {
		dest[i] = &values[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, false, err
	}

	truncatedAny := false
	for i, value := range values {
		converted, truncated := convertSQLCell(value, maxCellBytes)
		values[i] = converted
		truncatedAny = truncatedAny || truncated
	}
	return values, truncatedAny, nil
}

func convertSQLCell(value any, maxCellBytes int) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case []byte:
		return truncateSQLString(string(v), maxCellBytes)
	case time.Time:
		return v.Format(time.RFC3339), false
	case string:
		return truncateSQLString(v, maxCellBytes)
	default:
		return v, false
	}
}

func truncateSQLString(s string, maxBytes int) (string, bool) {
	if maxBytes == 0 || len(s) <= maxBytes {
		return s, false
	}
	if maxBytes < 0 {
		maxBytes = 0
	}
	end := maxBytes
	for end > 0 && !utf8.ValidString(s[:end]) {
		end--
	}
	return s[:end] + "…[truncated]", true
}
