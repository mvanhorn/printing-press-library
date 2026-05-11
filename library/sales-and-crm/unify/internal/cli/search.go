package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newSearchCmd runs FTS5 across every synced object's records and returns
// typed hits. The Unify Data API has no search endpoint — this is the local
// read primitive that replaces it.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var object string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across every synced record",
		Long: `Runs SQLite FTS5 across the local mirror's record_<object> tables.
Returns one row per hit with object_name, id, snippet, and a small selection
of high-gravity attributes (name, domain, email, created_at).

The Unify Data API has no search endpoint, so this only works against data
already pulled by 'sync'. If you get zero results, add to the watchlist
('unify-pp-cli watch add <object> --match k=v') and re-run sync.`,
		Example: strings.Trim(`
  unify-pp-cli search "gladly" --agent
  unify-pp-cli search "Retail" --object company --limit 20 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("search query is required"))
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()

			// Build FTS query. Plain phrase if no operators are present; otherwise
			// pass through (caller can use AND/OR/NEAR).
			ftsQuery := query
			if !strings.ContainsAny(query, "\"*:") && !containsBoolean(query) {
				ftsQuery = fmt.Sprintf("\"%s\"", strings.ReplaceAll(query, "\"", ""))
			}

			args2 := []any{ftsQuery}
			where := ""
			if object != "" {
				where = " AND object_name = ?"
				args2 = append(args2, object)
			}
			if limit <= 0 {
				limit = 50
			}
			q := fmt.Sprintf(`SELECT object_name, id, snippet(records_fts, 2, '[', ']', '…', 20) as snippet
				FROM records_fts
				WHERE body MATCH ?%s
				LIMIT %d`, where, limit)
			rows, err := s.DB.QueryContext(ctx, q, args2...)
			if err != nil {
				return apiErr(fmt.Errorf("fts query: %w", err))
			}
			// Drain rows first so the underlying connection is released before
			// we issue per-hit attr lookups. The store caps MaxOpenConns at 1
			// to keep writes serialized, so holding rows while issuing a new
			// query would deadlock.
			type ftsHit struct{ ObjectName, ID, Snippet string }
			var rawHits []ftsHit
			for rows.Next() {
				var h ftsHit
				if err := rows.Scan(&h.ObjectName, &h.ID, &h.Snippet); err != nil {
					rows.Close()
					return apiErr(err)
				}
				rawHits = append(rawHits, h)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return apiErr(err)
			}
			rows.Close()

			hits := make([]map[string]any, 0, len(rawHits))
			for _, rh := range rawHits {
				h := map[string]any{
					"object_name": rh.ObjectName,
					"id":          rh.ID,
					"snippet":     rh.Snippet,
				}
				if attrs := fetchRecordAttrs(ctx, s, rh.ObjectName, rh.ID); attrs != nil {
					for _, k := range []string{"name", "display_name", "domain", "email", "title", "stage"} {
						if v, ok := attrs[k]; ok {
							h[k] = v
						}
					}
				}
				hits = append(hits, h)
			}

			blob, _ := json.MarshalIndent(hits, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().StringVar(&object, "object", "", "Only search records of this object_name")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of hits")
	return cmd
}

func containsBoolean(s string) bool {
	u := strings.ToUpper(s)
	return strings.Contains(u, " AND ") || strings.Contains(u, " OR ") || strings.Contains(u, " NOT ") || strings.Contains(u, " NEAR(")
}

func fetchRecordAttrs(ctx context.Context, s *store.Store, objectName, id string) map[string]any {
	table := store.RecordTable(objectName)
	var attrs string
	q := fmt.Sprintf(`SELECT attrs FROM %q WHERE id = ?`, table)
	if err := s.DB.QueryRowContext(ctx, q, id).Scan(&attrs); err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(attrs), &m); err != nil {
		return nil
	}
	return m
}
