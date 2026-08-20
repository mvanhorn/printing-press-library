// Hand-authored offline-store commands: `search` (substring search over synced
// objects) and `sql` (read-only SELECT against the local store). Both read the
// SQLite database populated by `sync`. No generated header: preserved across
// `generate --force`.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/store"
)

// listFromLocalStore emits a resource list read entirely from the synced
// store, in the same envelope shape as the live list path.
func listFromLocalStore(cmd *cobra.Command, flags *rootFlags, resource, storeType string, limit int) error {
	st, err := store.OpenReadOnly(store.DefaultPath())
	if err != nil {
		return err
	}
	defer st.Close()
	items, err := st.List(cmd.Context(), storeType, limit)
	if err != nil {
		return fmt.Errorf("reading %s from local store: %w", resource, err)
	}
	return emitView(cmd, flags, saListView{Resource: resource, Count: len(items), Items: items})
}

// tryLocalFallback serves a resource list from the store after a live query
// failed under --data-source auto. Returns true only when the store had rows
// and was emitted successfully; the caller surfaces the original error
// otherwise. The degraded path is announced on stderr so it never looks like
// fresh live data.
func tryLocalFallback(cmd *cobra.Command, flags *rootFlags, resource, storeType string, limit int, liveErr error) bool {
	st, err := store.OpenReadOnly(store.DefaultPath())
	if err != nil {
		return false
	}
	defer st.Close()
	n, err := st.Count(cmd.Context(), storeType)
	if err != nil || n == 0 {
		return false
	}
	items, err := st.List(cmd.Context(), storeType, limit)
	if err != nil {
		return false
	}
	fmt.Fprintf(os.Stderr, "warning: live query failed (%v); serving %d %s from local store (run 'sync' to refresh)\n", liveErr, len(items), resource)
	return emitView(cmd, flags, saListView{Resource: resource, Count: len(items), Items: items}) == nil
}

// storeDBPath resolves the store location: an explicit --db flag wins,
// otherwise the STACKADAPT_DB env var / default path from the store package.
func storeDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return store.DefaultPath()
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var resourceType string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search synced StackAdapt objects offline by name or any field",
		Long: strings.Trim(`
Substring-search the local store populated by 'sync'. Matches against an
object's name and its full JSON, across advertisers, campaigns, campaign
groups, ads, and segments. Runs fully offline — no API calls.

Run 'stackadapt-pp-cli sync' first to populate the store.`, "\n"),
		Example: strings.Trim(`
  stackadapt-pp-cli search "spring" --agent
  stackadapt-pp-cli search acme --type advertisers
  stackadapt-pp-cli search 12345 --limit 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "search", "would search the local store")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			if resourceType != "" && resourceStoreType(resourceType) == "" {
				return usageErr(fmt.Errorf("unknown --type %q: valid types are advertisers, campaigns, campaign-groups, ads, segments", resourceType))
			}
			st, err := store.OpenReadOnly(storeDBPath(dbPath))
			if err != nil {
				return err
			}
			defer st.Close()

			storeType := ""
			if resourceType != "" {
				storeType = resourceStoreType(resourceType)
			}
			hits, err := st.Search(cmd.Context(), args[0], storeType, limit)
			if err != nil {
				return fmt.Errorf("searching local store: %w", err)
			}
			return emitView(cmd, flags, struct {
				Term  string      `json:"term"`
				Count int         `json:"count"`
				Hits  []store.Hit `json:"hits"`
			}{Term: args[0], Count: len(hits), Hits: hits})
		},
	}
	cmd.Flags().StringVar(&resourceType, "type", "", "Restrict to one resource: advertisers, campaigns, campaign-groups, ads, segments")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local store (default: STACKADAPT_DB or ~/.local/share/stackadapt-pp-cli/data.db)")
	return cmd
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL query against the local store",
		Long: strings.Trim(`
Execute a read-only SQL query (SELECT or WITH only) against the local SQLite
store populated by 'sync'. The database is opened read-only, so mutating
statements are rejected.

The synced objects live in the 'resources' table:
  resource_type, id, name, data (JSON), synced_at
Use SQLite's json_extract to reach into the stored JSON, e.g.
  json_extract(data, '$.channelType').

Run 'stackadapt-pp-cli sync' first to populate the store.`, "\n"),
		Example: strings.Trim(`
  stackadapt-pp-cli sql "SELECT resource_type, count(*) FROM resources GROUP BY resource_type"
  stackadapt-pp-cli sql "SELECT id, name FROM resources WHERE resource_type='campaigns' LIMIT 10"
  stackadapt-pp-cli sql "SELECT json_extract(data,'$.channelType') AS channel, count(*) FROM resources WHERE resource_type='ads' GROUP BY channel" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "sql", "would run a read-only SQL query against the local store")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL query is required"))
			}
			query := args[0]
			if err := ensureReadOnlySQL(query); err != nil {
				return usageErr(err)
			}
			st, err := store.OpenReadOnly(storeDBPath(dbPath))
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("running query: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			var records []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				rec := make(map[string]any, len(cols))
				for i, col := range cols {
					rec[col] = normalizeSQLValue(vals[i])
				}
				records = append(records, rec)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				if records == nil {
					records = []map[string]any{}
				}
				return emitView(cmd, flags, records)
			}
			// Plain output: one row per line, columns tab-joined, no header.
			// Keeps machine parsing unambiguous (the verify data-pipeline
			// probe greps this output for table names and counts).
			for _, rec := range records {
				cells := make([]string, len(cols))
				for i, col := range cols {
					cells[i] = fmt.Sprint(rec[col])
				}
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(cells, "\t"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local store (default: STACKADAPT_DB or ~/.local/share/stackadapt-pp-cli/data.db)")
	return cmd
}

// normalizeSQLValue converts a driver-scanned value into a JSON-friendly Go
// value. modernc.org/sqlite returns []byte for text columns.
func normalizeSQLValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return t
	}
}

// ensureReadOnlySQL rejects anything that is not a single SELECT/WITH query.
// Defense in depth on top of the read-only connection: gives a clear usage
// error before opening the store, and blocks multi-statement payloads.
func ensureReadOnlySQL(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("empty SQL query")
	}
	// Reject stacked statements: a trailing semicolon is fine, an interior
	// one means a second statement.
	if idx := strings.Index(trimmed, ";"); idx >= 0 && strings.TrimSpace(trimmed[idx+1:]) != "" {
		return fmt.Errorf("only a single read-only statement is allowed (no ';'-separated statements)")
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT/WITH queries are allowed; the local store is read-only")
	}
	return nil
}
