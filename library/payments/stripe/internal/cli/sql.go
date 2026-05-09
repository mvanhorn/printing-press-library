// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

// withCTEPrefix matches a leading "WITH ... AS (...)" common-table-expression
// chain so we can find the operative statement keyword that follows it.
var withCTEPrefix = regexp.MustCompile(`(?is)^\s*with\s+.+?\)\s*`)

// isReadOnlyQuery returns true when q is a SELECT (optionally preceded by one
// or more WITH/CTE clauses). Anything else — INSERT, UPDATE, DELETE, REPLACE,
// CREATE, DROP, ALTER, ATTACH, PRAGMA writes — is rejected. We rely on
// OpenReadOnly as the hard gate, but a friendly error here is cheaper than
// surfacing a SQLite "attempt to write a readonly database" string.
func isReadOnlyQuery(q string) bool {
	stripped := strings.TrimSpace(q)
	// Strip CTE prefixes — there may be more than one chained.
	for {
		m := withCTEPrefix.FindString(stripped)
		if m == "" {
			break
		}
		stripped = strings.TrimSpace(stripped[len(m):])
		// Some CTE chains use commas to attach more table expressions; those
		// don't change the operative-statement check, but a stray comma after
		// stripping means we have malformed SQL — let the engine handle it.
		stripped = strings.TrimLeft(stripped, ", ")
	}
	upper := strings.ToUpper(stripped)
	return strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "VALUES")
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var schemaOnly bool

	cmd := &cobra.Command{
		Use:   "sql [query...]",
		Short: "Run read-only SQLite queries against the local Stripe mirror",
		Long: `Run ad-hoc SELECT queries against the local SQLite database. The connection
is opened in read-only mode; INSERT, UPDATE, DELETE, REPLACE, and DDL are
rejected. All Stripe data lives in the generic 'resources' table — query
fields with json_extract(data, '$.field').`,
		Example: `  # List 5 customer emails
  stripe-pp-cli sql 'SELECT json_extract(data, "$.email") FROM resources WHERE resource_type="customers" LIMIT 5'

  # Show table layout
  stripe-pp-cli sql --schema`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !schemaOnly && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' to populate it.", path, err))
			}
			defer db.Close()

			if schemaOnly {
				return printSQLSchema(cmd, flags, db.DB())
			}

			query := strings.Join(args, " ")
			if !isReadOnlyQuery(query) {
				return usageErr(fmt.Errorf("only read-only SELECT/WITH queries are accepted; got: %s", strings.SplitN(strings.TrimSpace(query), "\n", 2)[0]))
			}

			return runSQLQuery(cmd, flags, db.DB(), query)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")
	cmd.Flags().BoolVar(&schemaOnly, "schema", false, "Print the schema (tables and column hints) instead of running a query")

	return cmd
}

// runSQLQuery executes query and emits the rows as []map[string]any via the
// shared printJSONFiltered pipeline.
func runSQLQuery(cmd *cobra.Command, flags *rootFlags, db *sql.DB, query string) error {
	rows, err := db.QueryContext(cmd.Context(), query)
	if err != nil {
		return usageErr(fmt.Errorf("query failed: %w", err))
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return apiErr(fmt.Errorf("reading columns: %w", err))
	}

	results := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return apiErr(fmt.Errorf("scanning row: %w", err))
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalizeSQLValue(vals[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return apiErr(fmt.Errorf("iterating rows: %w", err))
	}

	return printJSONFiltered(cmd.OutOrStdout(), results, flags)
}

// normalizeSQLValue collapses sqlite driver scan types ([]byte → string) so the
// JSON encoder produces readable strings rather than base64.
func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return v
	}
}

// printSQLSchema lists tables and per-table column hints derived from PRAGMA
// table_info, plus a one-line note about the resources/JSON storage shape.
func printSQLSchema(cmd *cobra.Command, flags *rootFlags, db *sql.DB) error {
	type col struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type tbl struct {
		Name    string `json:"name"`
		Columns []col  `json:"columns"`
	}

	tables := make([]tbl, 0)
	tableRows, err := db.QueryContext(cmd.Context(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return apiErr(fmt.Errorf("listing tables: %w", err))
	}
	defer tableRows.Close()
	var names []string
	for tableRows.Next() {
		var n string
		if err := tableRows.Scan(&n); err != nil {
			return apiErr(err)
		}
		names = append(names, n)
	}
	if err := tableRows.Err(); err != nil {
		return apiErr(err)
	}

	for _, name := range names {
		colRows, err := db.QueryContext(cmd.Context(), fmt.Sprintf("PRAGMA table_info(%q)", name))
		if err != nil {
			continue
		}
		var cols []col
		for colRows.Next() {
			var cid int
			var cname, ctype string
			var notnull, pk int
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk); err == nil {
				cols = append(cols, col{Name: cname, Type: ctype})
			}
		}
		colRows.Close()
		tables = append(tables, tbl{Name: name, Columns: cols})
	}

	out := map[string]any{
		"hint":   "Stripe entities live in the 'resources' table as (resource_type, id, data JSON). Use json_extract(data, '$.field').",
		"tables": tables,
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}
