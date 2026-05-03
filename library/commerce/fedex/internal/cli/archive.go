// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newArchiveCmd(flags *rootFlags) *cobra.Command {
	var (
		service string
		account string
		since   string
		limit   int
	)
	cmd := &cobra.Command{
		Use:         "archive [query]",
		Short:       "Search the local shipment archive (FTS5 over recipient, reference, tracking)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  fedex-pp-cli archive
  fedex-pp-cli archive "acme corp"
  fedex-pp-cli archive --service FEDEX_GROUND --since 720h --limit 25
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()

			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			if limit <= 0 {
				limit = 50
			}

			ctx := context.Background()
			rows, err := runArchiveQuery(ctx, st.DB(), query, service, account, since, limit)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"TRACKING", "RECIPIENT", "CITY", "STATE", "SERVICE", "NET", "CREATED"}
				tableRows := make([][]string, 0, len(rows))
				for _, r := range rows {
					tableRows = append(tableRows, []string{
						getString(r, "tracking_number"),
						getString(r, "recipient_name"),
						getString(r, "recipient_city"),
						getString(r, "recipient_state"),
						getString(r, "service_type"),
						fmt.Sprintf("%.2f", getFloat(r, "net_charge_amount")),
						getString(r, "created_at"),
					})
				}
				return flags.printTable(cmd, headers, tableRows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&service, "service", "", "Filter by service_type")
	cmd.Flags().StringVar(&account, "account", "", "Filter by account")
	cmd.Flags().StringVar(&since, "since", "", "Only shipments created within this duration (e.g. 24h, 720h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}

// ftsQuoteQuery turns a free-form user query into a safe FTS5 MATCH
// expression. FTS5 treats ASCII single/double quotes, parentheses, and
// `:` as syntactic, so a query like `'warehouse 47'` (or `acme corp`)
// raises "fts5: syntax error". We wrap each whitespace-separated term in
// FTS5 double quotes (with internal double quotes doubled) and join them
// with implicit AND. Single-quoted phrases are stripped so users can
// quote in shell habits without breaking the query.
func ftsQuoteQuery(q string) string {
	trimmed := strings.TrimSpace(q)
	if trimmed == "" {
		return ""
	}
	// If the user wrapped the whole query in matching single/double quotes,
	// treat the inside as a single phrase.
	if (strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") && len(trimmed) >= 2) ||
		(strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) && len(trimmed) >= 2) {
		inner := trimmed[1 : len(trimmed)-1]
		return `"` + strings.ReplaceAll(inner, `"`, `""`) + `"`
	}
	parts := strings.Fields(trimmed)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, `"'`)
		if p == "" {
			continue
		}
		out = append(out, `"`+strings.ReplaceAll(p, `"`, `""`)+`"`)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, " AND ")
}

func runArchiveQuery(ctx context.Context, db *sql.DB, query, service, account, since string, limit int) ([]map[string]any, error) {
	var (
		clauses []string
		args    []any
		sqlStmt string
	)
	if query != "" {
		sqlStmt = `SELECT s.* FROM shipments s WHERE s.id IN (SELECT rowid FROM shipments_fts WHERE shipments_fts MATCH ?)`
		args = append(args, ftsQuoteQuery(query))
	} else {
		sqlStmt = `SELECT s.* FROM shipments s WHERE 1=1`
	}
	if service != "" {
		clauses = append(clauses, "s.service_type = ?")
		args = append(args, service)
	}
	if account != "" {
		clauses = append(clauses, "s.account = ?")
		args = append(args, account)
	}
	if since != "" {
		dur, err := time.ParseDuration(since)
		if err == nil {
			clauses = append(clauses, "s.created_at >= ?")
			args = append(args, time.Now().Add(-dur))
		}
	}
	for _, c := range clauses {
		sqlStmt += " AND " + c
	}
	sqlStmt += " ORDER BY s.created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, sqlStmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = normalizeSQLValue(vals[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return v
	}
}

func getString(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func getFloat(m map[string]any, k string) float64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	}
	return 0
}
