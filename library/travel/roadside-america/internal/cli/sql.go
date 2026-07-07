package cli

import (
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var sqlMutatingRe = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|replace|attach|detach|pragma|vacuum|reindex|truncate)\b`)

// isReadOnlySQL accepts a single SELECT/WITH statement and rejects anything
// that could mutate the local store or chain extra statements.
func isReadOnlySQL(q string) bool {
	t := strings.ToLower(strings.TrimSpace(q))
	t = strings.TrimSuffix(strings.TrimSpace(t), ";")
	if strings.Contains(t, ";") {
		return false
	}
	if !strings.HasPrefix(t, "select") && !strings.HasPrefix(t, "with") {
		return false
	}
	return !sqlMutatingRe.MatchString(t)
}

type sqlResultView struct {
	Source string           `json:"source"`
	Count  int              `json:"count"`
	Rows   []map[string]any `json:"rows"`
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sql <SELECT ...>",
		Short: "Run a read-only SQL query against the local attraction cache",
		Long: strings.Trim(`
Run a read-only SQL (SELECT/WITH) query against the local SQLite cache. Cached
attractions live in the 'resources' table as JSON:

  id            TEXT     attraction id
  resource_type TEXT     'attraction' (list rows) or 'detail' (full writeups)
  data          JSON     use json_extract(data, '$.field'): name, city, state,
                         street, source_url, categories, cached_at
  updated_at    DATETIME when the row was cached

Only single read-only statements are permitted.`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli sql "SELECT json_extract(data,'$.name') AS name, json_extract(data,'$.state') AS state FROM resources WHERE resource_type='attraction' LIMIT 10"
  roadside-america-pp-cli sql "SELECT json_extract(data,'$.state') AS state, COUNT(*) AS n FROM resources WHERE resource_type='attraction' GROUP BY state ORDER BY n DESC"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a read-only SQL query against the local cache")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SELECT query is required"))
			}
			query := strings.Join(args, " ")
			if !isReadOnlySQL(query) {
				return usageErr(fmt.Errorf("only a single read-only SELECT/WITH query is allowed"))
			}

			s, err := openRoadsideStoreReadOnly()
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.Query(query)
			if err != nil {
				return apiErr(fmt.Errorf("query failed: %w", err))
			}
			defer rows.Close()
			result, err := scanRowsToMaps(rows)
			if err != nil {
				return apiErr(fmt.Errorf("reading rows: %w", err))
			}
			view := sqlResultView{Source: "local cache (resources table)", Count: len(result), Rows: result}
			// Plain text by default (pipe- and tooling-friendly: one row per line,
			// tab-separated, with a column header). JSON only on explicit request.
			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" || flags.quiet {
				return flags.printJSON(cmd, view)
			}
			return emitSQLPlain(cmd.OutOrStdout(), result)
		},
	}
	return cmd
}

// emitSQLPlain writes query results as a header row plus tab-separated value
// rows. Deterministic column order (sorted) so output is stable across runs.
func emitSQLPlain(w io.Writer, rows []map[string]any) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "(0 rows)")
		return nil
	}
	cols := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	fmt.Fprintln(w, strings.Join(cols, "\t"))
	for _, r := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = fmt.Sprintf("%v", r[c])
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return nil
}

// scanRowsToMaps scans arbitrary result columns into a slice of maps, decoding
// []byte columns (TEXT/JSON) into strings for clean JSON output.
func scanRowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
