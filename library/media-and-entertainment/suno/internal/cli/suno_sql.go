// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `sql` — run read-only SQL against the local clip store. Only a single
// SELECT / WITH...SELECT statement is permitted. Reads the local SQLite store
// only; no network and no auth. Read-only.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

func newSunoSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SELECT query against the local clip store",
		Long: "Run read-only SQL against the local store. Only a single SELECT (or " +
			"WITH ... SELECT) statement is accepted; any write or DDL statement " +
			"(INSERT/UPDATE/DELETE/DROP/ALTER/CREATE/ATTACH/PRAGMA/...) or multiple " +
			"statements are rejected.\n\n" +
			"Tables: clips, lyrics, personas, workspace, billing, resources, sync_state.",
		Example:     "  suno-pp-cli sql \"SELECT count(*) AS n FROM clips\"\n  suno-pp-cli sql \"SELECT id, title FROM clips ORDER BY play_count DESC LIMIT 5\" --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a SQL query is required: sql \"SELECT ...\""))
			}
			query := args[0]
			if err := validateReadOnlySQL(query); err != nil {
				return usageErr(err)
			}

			// Open read-only: mode=ro enforces read-only at the SQLite driver
			// level, rejecting direct and CTE-wrapped writes even if a query
			// were to slip past validateReadOnlySQL. The app-level validator
			// stays as a friendly pre-check.
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "")
			hintIfStale(cmd, db, "", flags.maxAge)

			results, err := runReadOnlyQuery(db, query)
			if err != nil {
				return fmt.Errorf("executing query: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// sqlCommentRE strips -- line comments and /* */ block comments so the
// statement validator can't be fooled by hidden writes inside comments.
var sqlCommentRE = regexp.MustCompile(`--[^\n]*|/\*[\s\S]*?\*/`)

// sqlForbiddenLeadingRE matches leading keywords of mutating / DDL / control
// statements that are never allowed.
var sqlForbiddenLeadingRE = regexp.MustCompile(`(?i)^(insert|update|delete|replace|drop|alter|create|attach|detach|pragma|vacuum|reindex|analyze|begin|commit|rollback|savepoint|release)\b`)

// validateReadOnlySQL accepts exactly one SELECT or WITH...SELECT statement.
// It rejects mutating/DDL statements, multiple statements (trailing semicolon
// + more SQL), and any statement that doesn't start with SELECT or WITH. A
// WITH prefix must still resolve to a read query — if it contains a forbidden
// write keyword as a statement lead it is rejected (CTE write-targets like
// "WITH x AS (...) INSERT" never start with SELECT/WITH at the outer level so
// the leading check already blocks the simple case; the embedded-keyword scan
// catches "WITH ... DELETE FROM ...").
func validateReadOnlySQL(raw string) error {
	stripped := sqlCommentRE.ReplaceAllString(raw, " ")
	trimmed := strings.TrimSpace(stripped)
	if trimmed == "" {
		return fmt.Errorf("empty query")
	}

	// Reject multiple statements: a semicolon followed by any non-whitespace.
	if idx := strings.Index(trimmed, ";"); idx >= 0 {
		if strings.TrimSpace(trimmed[idx+1:]) != "" {
			return fmt.Errorf("only a single statement is allowed")
		}
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	// Collapse every whitespace run to a single space so the write-keyword
	// scan below cannot be bypassed by a newline or tab after the keyword
	// (e.g. "WITH t AS (SELECT 1) DELETE\nFROM clips").
	lower := strings.Join(strings.Fields(strings.ToLower(trimmed)), " ")
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("only read-only SELECT (or WITH ... SELECT) queries are allowed")
	}
	if sqlForbiddenLeadingRE.MatchString(trimmed) {
		return fmt.Errorf("only read-only SELECT queries are allowed; mutating/DDL statements are rejected")
	}

	// Scan for forbidden statement keywords appearing at a statement boundary
	// inside a WITH clause (e.g. "WITH t AS (...) DELETE FROM clips").
	if strings.HasPrefix(lower, "with") {
		writeKeywords := []string{"insert ", "update ", "delete ", "replace ", "drop ", "alter ", "create ", "attach ", "pragma "}
		for _, kw := range writeKeywords {
			if strings.Contains(lower, kw) {
				return fmt.Errorf("only read-only SELECT queries are allowed; the WITH query contains a write/DDL keyword (%s)", strings.TrimSpace(kw))
			}
		}
	}
	return nil
}

// runReadOnlyQuery executes a validated SELECT and returns rows as a slice of
// column->value maps. Values are scanned NULL-safe via *any and []byte is
// decoded to string so JSON output is readable.
func runReadOnlyQuery(db *store.Store, query string) ([]map[string]any, error) {
	rows, err := db.DB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0)
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			switch v := cells[i].(type) {
			case []byte:
				row[c] = string(v)
			default:
				row[c] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
