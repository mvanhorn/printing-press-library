// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Framework command `sql`, restored by hand. It was accidentally listed in
// research.json novel_features in an earlier generation, which made the
// generator emit a novel-command TODO stub that SHADOWED the real framework sql
// (playbook Addendum 2026-07-07). research.json is now clean, but a hand-built
// fork must never be regenerated, so the real command is restored here directly.
//
// mcp:hidden — a typed `sql` MCP tool already exists (internal/mcp/tools.go),
// so this CLI command is hidden from the cobratree mirror to avoid a duplicate
// tool, matching how search.go is wired.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type sqlReport struct {
	Query   string                   `json:"query"`
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

func newNovelSqlCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run read-only SQL over the local SQLite corpus for arbitrary analytics (post frequency, audience mix, longest posts)",
		Long: `Execute a single read-only SELECT/WITH statement against the local SQLite
corpus populated by 'substack-reader-pp-cli archive'. Read-only is enforced two ways:
the query is gated to a single SELECT or WITH statement (comment- and
multi-statement-injection resistant), and the store is opened mode=ro so the
driver rejects any write that slips past the parser.

Archived posts live in the 'posts' resource type:
  substack-reader-pp-cli sql "SELECT COUNT(*) AS n FROM resources WHERE resource_type='posts'"
  substack-reader-pp-cli sql "SELECT json_extract(data,'$.audience') AS audience, COUNT(*) AS n FROM resources WHERE resource_type='posts' GROUP BY audience"`,
		Example:     "  substack-reader-pp-cli sql \"SELECT json_extract(data,'$.audience') AS audience, COUNT(*) FROM resources WHERE resource_type='posts' GROUP BY audience\"",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if err := validateReadOnlySQL(query); err != nil {
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// openStoreForRead opens mode=ro and returns (nil,nil) when no DB
			// exists yet — a friendlier "archive first" than a raw open error.
			db, err := openStoreForRead(ctx, "substack-reader-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local corpus: %w", err)
			}
			if db == nil {
				return fmt.Errorf("no local corpus yet — run 'substack-reader-pp-cli archive <publication>' first")
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("running query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Errorf("reading columns: %w", err)
			}
			report := &sqlReport{
				Query:   query,
				Columns: cols,
				Rows:    make([]map[string]interface{}, 0),
			}
			for rows.Next() {
				dest := make([]interface{}, len(cols))
				ptrs := make([]interface{}, len(cols))
				for i := range dest {
					ptrs[i] = &dest[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				row := make(map[string]interface{}, len(cols))
				for i, c := range cols {
					row[c] = normalizeSQLValue(dest[i])
				}
				report.Rows = append(report.Rows, row)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}
			report.Count = len(report.Rows)

			b, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}

// validateReadOnlySQL gates the CLI sql command to a single read-only
// statement. This mirrors the MCP sql tool's validateReadOnlyQuery
// (internal/mcp/tools.go) — the two cannot share code without an
// mcp<->cli import cycle, so the gate is duplicated deliberately so both
// agent-facing surfaces enforce the SAME contract.
//
// It rejects multi-statement input, then allows only a leading SELECT or WITH
// AFTER stripping the whitespace, line/block comments, and semicolons SQLite
// itself skips before parsing. A naive prefix check is bypassable by prefixing
// "/* x */" or appending "; ATTACH DATABASE …"; combined with mode=ro one layer
// down (which does NOT block ATTACH or VACUUM INTO on its own), either bypass
// would enable silent exfiltration to an attacker-chosen path.
func validateReadOnlySQL(query string) error {
	stripped := stripLeadingSQLNoise(query)
	if hasTrailingSQLStatement(stripped) {
		return fmt.Errorf("only a single SELECT or WITH statement is allowed")
	}
	upper := strings.ToUpper(stripped)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only read-only SELECT or WITH queries are allowed")
	}
	return nil
}

// stripLeadingSQLNoise removes leading whitespace, SQL line comments (-- to end
// of line), block comments (/* ... */), and statement separators (;) so the
// gate sees the same first keyword SQLite will parse.
func stripLeadingSQLNoise(query string) string {
	for {
		query = strings.TrimLeft(query, " \t\r\n;")
		switch {
		case strings.HasPrefix(query, "--"):
			if idx := strings.IndexByte(query, '\n'); idx >= 0 {
				query = query[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(query, "/*"):
			if idx := strings.Index(query[2:], "*/"); idx >= 0 {
				query = query[2+idx+2:]
				continue
			}
			return ""
		default:
			return query
		}
	}
}

// hasTrailingSQLStatement reports whether query contains a statement terminator
// followed by more executable SQL. A trailing semicolon is allowed; a second
// statement is not. Semicolons inside string literals, quoted identifiers,
// bracket identifiers, and comments are ignored to match SQLite's parser shape
// closely enough for this security gate.
func hasTrailingSQLStatement(query string) bool {
	inSingle := false
	inDouble := false
	inBacktick := false
	inBracket := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}

		switch {
		case inLineComment:
			if ch == '\n' {
				inLineComment = false
			}
			continue
		case inBlockComment:
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		case inSingle:
			if ch == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		case inDouble:
			if ch == '"' {
				if next == '"' {
					i++
					continue
				}
				inDouble = false
			}
			continue
		case inBacktick:
			if ch == '`' {
				if next == '`' {
					i++
					continue
				}
				inBacktick = false
			}
			continue
		case inBracket:
			if ch == ']' {
				inBracket = false
			}
			continue
		}

		switch {
		case ch == '-' && next == '-':
			inLineComment = true
			i++
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
		case ch == '\'':
			inSingle = true
		case ch == '"':
			inDouble = true
		case ch == '`':
			inBacktick = true
		case ch == '[':
			inBracket = true
		case ch == ';':
			if stripLeadingSQLNoise(query[i+1:]) != "" {
				return true
			}
			return false
		}
	}
	return false
}

// normalizeSQLValue converts SQLite's interface{} returns ([]byte for text,
// int64 for ints, etc.) into JSON-friendly values, parsing JSON-BLOB columns
// (the store keeps each resource's payload in a JSON `data` column) so nested
// objects render as structure rather than an escaped string.
func normalizeSQLValue(v interface{}) interface{} {
	switch x := v.(type) {
	case []byte:
		s := string(x)
		var parsed interface{}
		if json.Unmarshal(x, &parsed) == nil {
			return parsed
		}
		return s
	default:
		return v
	}
}
