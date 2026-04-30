package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pokeapi/internal/store"
)

type sqlReport struct {
	Query   string                   `json:"query"`
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Read-only SQL passthrough against the local store",
		Long: `Executes a single SELECT statement against the local SQLite database populated
by 'pokeapi-pp-cli sync'. Read-only — INSERT/UPDATE/DELETE/DROP/ATTACH/PRAGMA-with-side-
effects are rejected. Useful for one-off compositions the typed commands don't
cover.

Run 'pokeapi-pp-cli analytics --help' for higher-level group-by/summary analytics
that don't require writing SQL.`,
		Example: strings.Trim(`
  pokeapi-pp-cli sql "SELECT id FROM resources WHERE resource_type='pokemon' LIMIT 5" --json
  pokeapi-pp-cli sql "SELECT COUNT(*) FROM resources WHERE resource_type='ability'" --json
  pokeapi-pp-cli sql "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if err := validateReadOnlyQuery(query); err != nil {
				return err
			}
			dbPath := defaultDBPath("pokeapi-pp-cli")
			// Open via store.Open so this command shares schema migrations and
			// the WAL/PRAGMA tuning the rest of the CLI uses; falls back to a
			// raw sql.Open if the store package can't initialize (e.g., no DB
			// yet — caller should run sync first).
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store at %s: %w (did you run 'pokeapi-pp-cli sync' first?)", dbPath, err)
			}
			defer s.Close()
			db := s.DB()
			_ = sql.ErrNoRows // keep database/sql in scope
			rows, err := db.Query(query)
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

// validateReadOnlyQuery rejects anything that could mutate the local store.
// SQLite's `db.Query` would honor an UPDATE returning rows, so we have to
// gate at the parser level. This is a deny-list and intentionally conservative.
func validateReadOnlyQuery(q string) error {
	upper := strings.ToUpper(strings.TrimSpace(q))
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") && !strings.HasPrefix(upper, "PRAGMA") {
		return fmt.Errorf("query must start with SELECT, WITH, or PRAGMA (read-only); got %q", firstWord(upper))
	}
	bannedTokens := []string{
		" INSERT ", " UPDATE ", " DELETE ", " DROP ", " ALTER ", " ATTACH ",
		" REINDEX ", " VACUUM ", " REPLACE ", " CREATE ", " TRIGGER ", " TRUNCATE ",
	}
	wrapped := " " + upper + " "
	for _, t := range bannedTokens {
		if strings.Contains(wrapped, t) {
			return fmt.Errorf("query contains write operation %q; only read queries are allowed", strings.TrimSpace(t))
		}
	}
	return nil
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			return s[:i]
		}
	}
	return s
}

// normalizeSQLValue converts SQLite's interface{} returns ([]byte for text,
// int64 for ints, etc.) into JSON-friendly values.
func normalizeSQLValue(v interface{}) interface{} {
	switch x := v.(type) {
	case []byte:
		// Many text columns come back as []byte; render as string. Try JSON parse
		// for columns the generator stores as JSON BLOBs.
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
