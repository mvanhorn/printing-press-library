// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// `obsidian-pp-cli vault-sql` — raw SQL against the local obsidian mirror.
// Opens the SQLite store read-only so even a SELECT that's actually a
// DELETE-disguised-as-CTE is rejected at the driver level. Result rows
// stream out as a JSON array of objects (column->value) or as a simple
// tab-separated table for human-friendly mode.
//
// Named "vault-sql" rather than "sql" to avoid colliding with the
// Press's typed `sql` shortcut, which compiles SELECT-style queries
// against the generic resources table (empty for obsidian).

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newVaultSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "vault-sql <query>",
		Short: "Run a SELECT query against the local obsidian mirror",
		Long: `Run a SELECT statement against the obsidian-specific tables in the
local mirror (notes, obsidian_tags, obsidian_links, frontmatter_kv,
vault_sync_state). The store is opened read-only so writes are rejected
at the driver level.

The mirror schema:
  notes (id, path, title, created_at, modified_at, word_count,
         content_hash, frontmatter_json)
  obsidian_tags (note_id, tag)
  obsidian_links (source_id, target_path, link_type, resolved)
  frontmatter_kv (note_id, key, value)
  vault_sync_state (id, vault_path, last_sync_at, notes_synced)`,
		Example: `  # Top 10 most-linked notes
  obsidian-pp-cli vault-sql "SELECT n.title, count(l.source_id) AS incoming \
    FROM notes n JOIN obsidian_links l ON l.target_path = n.path \
    GROUP BY n.id ORDER BY incoming DESC LIMIT 10"

  # Notes with a specific tag
  obsidian-pp-cli vault-sql "SELECT n.path FROM notes n \
    JOIN obsidian_tags t ON t.note_id = n.id WHERE t.tag = 'project'"`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("query is required"))
			}
			query := strings.TrimSpace(args[0])
			if !looksLikeSelect(query) {
				return usageErr(fmt.Errorf("only SELECT/WITH queries are accepted"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("obsidian-pp-cli")
			}
			// Ensure schema exists (cheap) before opening read-only —
			// otherwise queries against an unsynced db fail with
			// "no such table".
			{
				rw, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				if err := rw.EnsureObsidianSchema(); err != nil {
					rw.Close()
					return err
				}
				rw.Close()
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database (read-only): %w", err)
			}
			defer db.Close()

			rows, err := db.DB().Query(query)
			if err != nil {
				return fmt.Errorf("running query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			var items []map[string]any
			counter := 0
			for rows.Next() {
				if limit > 0 && counter >= limit {
					break
				}
				raw := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range raw {
					ptrs[i] = &raw[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				row := map[string]any{}
				for i, c := range cols {
					row[c] = normalizeScanned(raw[i])
				}
				items = append(items, row)
				counter++
			}
			// rows.Err surfaces driver-level errors that happen after the
			// last successful Next() — without this check, a truncated
			// stream silently looks like a complete short result set.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			// Reuse the already-open read-only handle for the staleness
			// hint. The prior approach opened a second `*store.Store` via
			// mustOpenForWarning() and never closed it — under the
			// long-running MCP server each `vault-sql` invocation leaked
			// one DB handle.
			emitStalenessWarning(cmd, db)

			if flags.asJSON {
				out, _ := json.MarshalIndent(map[string]any{
					"columns": cols,
					"count":   len(items),
					"rows":    items,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "0 rows.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cols, "\t"))
			for _, item := range items {
				vals := make([]string, len(cols))
				for i, c := range cols {
					vals[i] = fmt.Sprintf("%v", item[c])
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(vals, "\t"))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d rows\n", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum rows to return")
	return cmd
}

// looksLikeSelect rejects anything that doesn't read as a SELECT or CTE.
// The driver-level read-only mode would already reject writes, but
// failing early at the CLI gives a cleaner error message and protects
// against odd corner cases (e.g., PRAGMA writes, ATTACH DATABASE).
//
// Accepts the keyword followed by any non-identifier character (space,
// tab, newline, `(`, `*`, `"`) so compact forms like `SELECT*FROM ...`
// and CTE shapes like `WITH(...)` aren't rejected at the gate.
func looksLikeSelect(q string) bool {
	lower := strings.ToLower(strings.TrimSpace(q))
	for _, prefix := range []string{"select", "with"} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := lower[len(prefix):]
		if rest == "" {
			continue
		}
		r := rest[0]
		// Any non-[a-z0-9_] after the keyword means we're past the
		// identifier and into the rest of the query.
		isIdent := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !isIdent {
			return true
		}
	}
	return false
}

// normalizeScanned converts driver-returned raw column values into
// JSON-friendly Go types. modernc.org/sqlite returns []byte for TEXT/BLOB
// columns; strings encode cleaner in JSON.
func normalizeScanned(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}

// Mark the sql import as used; some callers expect it transitively.
var _ = sql.ErrNoRows
