package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newSQLCmd runs read-only SQL against the local mirror. Joins across
// record_<object> tables let you ask cross-source questions the API can't
// answer in one call.
func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxRows int

	cmd := &cobra.Command{
		Use:   "sql [query]...",
		Args:  cobra.ArbitraryArgs,
		Short: "Run a read-only SELECT against the local SQLite mirror",
		Long: `The local store mirrors every synced record into a per-object table named
record_<object> (e.g. record_company, record_salesforce_account). Attributes
live inside the 'attrs' JSON column — use json_extract(attrs, '$.<name>')
to pull specific fields.

Tables exposed:
  - objects, attributes, attribute_options, schema_snapshots, watchlist
  - record_<object> for every synced object_name (use SELECT name FROM
    sqlite_master WHERE type='table' to enumerate)

Only SELECT and WITH (read-only) statements are allowed.`,
		Example: strings.Trim(`
  unify-pp-cli sql "SELECT api_name FROM objects"
  unify-pp-cli sql "SELECT id, json_extract(attrs,'$.domain') as domain FROM record_company LIMIT 5"
  unify-pp-cli sql "SELECT json_extract(c.attrs,'$.name') name, json_extract(a.attrs,'$.industry') industry FROM record_company c LEFT JOIN record_salesforce_account a ON json_extract(c.attrs,'$.domain') = json_extract(a.attrs,'$.website') WHERE json_extract(a.attrs,'$.industry')='Retail'"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if err := assertReadOnlySQL(query); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			rows, err := s.DB.QueryContext(ctx, query)
			if err != nil {
				return apiErr(fmt.Errorf("sql: %w", err))
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return apiErr(err)
			}
			out := make([]map[string]any, 0, 16)
			rowCount := 0
			for rows.Next() {
				if maxRows > 0 && rowCount >= maxRows {
					break
				}
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return apiErr(err)
				}
				row := map[string]any{}
				for i, c := range cols {
					row[c] = stringifyDBValue(vals[i])
				}
				out = append(out, row)
				rowCount++
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().IntVar(&maxRows, "max-rows", 1000, "Cap on returned rows (0 = unlimited)")
	return cmd
}

// assertReadOnlySQL rejects anything other than SELECT/WITH at the top
// level. SQLite would otherwise happily execute UPDATE/DELETE/etc.
func assertReadOnlySQL(q string) error {
	u := strings.ToUpper(strings.TrimSpace(q))
	for _, bad := range []string{"INSERT ", "UPDATE ", "DELETE ", "REPLACE ", "DROP ", "ALTER ", "CREATE ", "TRUNCATE ", "ATTACH ", "DETACH ", "PRAGMA ", "VACUUM"} {
		if strings.HasPrefix(u, bad) || strings.Contains(u, "; "+bad) {
			return fmt.Errorf("sql: only read-only SELECT/WITH statements are allowed; refusing %q", strings.TrimSpace(bad))
		}
	}
	if !strings.HasPrefix(u, "SELECT ") && !strings.HasPrefix(u, "WITH ") {
		return fmt.Errorf("sql: query must start with SELECT or WITH")
	}
	return nil
}

func stringifyDBValue(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}
