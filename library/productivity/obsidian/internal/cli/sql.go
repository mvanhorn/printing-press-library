package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run a read-only SQL query against the local store.",
		Long: "Run an arbitrary read-only SQL query against the synced SQLite store.\n" +
			"Only SELECT statements are permitted. Useful tables:\n" +
			"  notes(path, type, date, description, status, mtime, layer, body_text, body_hash, has_fm)\n" +
			"  facts(id, parent_note_path, fact, category, timestamp, status, source, decision_trace_id, storage)\n" +
			"  links(from_path, to_target, resolved_path)\n" +
			"  tags(path, tag, source)\n" +
			"  frontmatter_fields(path, key, value)",
		Example: "  obsidian-pp-cli sql \"SELECT path, description FROM notes WHERE type='meeting' LIMIT 10\"\n" +
			"  obsidian-pp-cli sql \"SELECT decision_trace_id, COUNT(*) FROM facts GROUP BY decision_trace_id\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			if !isReadOnlyQuery(query) {
				return usageErr(fmt.Errorf("only SELECT statements are permitted (rejected: %q)", firstWord(query)))
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			rows, err := vc.S.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return apiErr(fmt.Errorf("query: %w", err))
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return apiErr(err)
			}
			var out []map[string]interface{}
			for rows.Next() {
				vals := make([]interface{}, len(cols))
				ptrs := make([]interface{}, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return apiErr(err)
				}
				row := map[string]interface{}{}
				for i, c := range cols {
					row[c] = normalizeSQLValue(vals[i])
				}
				out = append(out, row)
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no rows)")
				return nil
			}
			// Human-friendly table.
			for _, row := range out {
				for _, c := range cols {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%v  ", c, row[c])
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}

// isReadOnlyQuery returns true only when the first significant token is SELECT or WITH (CTE).
// Tolerates leading quotes left over from shell parsing.
func isReadOnlyQuery(q string) bool {
	q = strings.TrimSpace(q)
	q = strings.TrimLeft(q, "'\"")
	if strings.HasPrefix(q, "--") {
		return false
	}
	w := strings.ToUpper(firstWord(q))
	return w == "SELECT" || w == "WITH"
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "'\"")
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '(' {
			return s[:i]
		}
	}
	return s
}

func normalizeSQLValue(v interface{}) interface{} {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case sql.RawBytes:
		return string(t)
	default:
		return t
	}
}
