package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/store"

	"github.com/spf13/cobra"
)

// newAuditScoresCmd flags records where two numeric attributes diverge
// beyond a threshold. Powers scoring sanity checks (e.g. the "auto-deduct
// 50 points" Unify+Salesforce divergence).
func newAuditScoresCmd(flags *rootFlags) *cobra.Command {
	var dbPath, object string
	var fields []string
	var threshold float64
	var limit int

	cmd := &cobra.Command{
		Use:   "audit-scores",
		Short: "Flag records where two numeric attributes diverge beyond a threshold",
		Long: `Reads the local mirror's record_<object> table, extracts two numeric
attributes per record, and reports rows where ABS(field_a - field_b) > threshold.

Use this to catch scoring drift between Unify-native and Salesforce-mirrored
score fields, false-scoring auto-deducts, or any pair of supposed-to-agree
numbers that have started disagreeing.`,
		Example: strings.Trim(`
  unify-pp-cli audit-scores --object company --field unify_score --field salesforce_lead_score --threshold 50 --agent
  unify-pp-cli audit-scores --object salesforce_account --field annual_revenue --field arr --threshold 1000000
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if object == "" || len(fields) < 2 {
				return usageErr(fmt.Errorf("--object and at least two --field args are required"))
			}
			fieldA, fieldB := fields[0], fields[1]
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			s, err := store.Open(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer s.Close()
			table := store.RecordTable(object)
			q := fmt.Sprintf(`SELECT id, json_extract(attrs, '$."%s"') as a, json_extract(attrs, '$."%s"') as b, json_extract(attrs, '$."name"') as name FROM %q`,
				escapeJSONKey(fieldA), escapeJSONKey(fieldB), table)
			rows, err := s.DB.QueryContext(ctx, q)
			if err != nil {
				return apiErr(fmt.Errorf("query %s: %w. Run 'sync' first if the table is missing.", table, err))
			}
			defer rows.Close()
			type div struct {
				ID    string  `json:"id"`
				Name  string  `json:"name,omitempty"`
				A     float64 `json:"a"`
				B     float64 `json:"b"`
				Delta float64 `json:"delta"`
			}
			var diverged []div
			scanned, withBoth := 0, 0
			for rows.Next() {
				var id, name string
				var av, bv any
				if err := rows.Scan(&id, &av, &bv, &name); err != nil {
					return apiErr(err)
				}
				scanned++
				a, aOK := numericOf(av)
				b, bOK := numericOf(bv)
				if !aOK || !bOK {
					continue
				}
				withBoth++
				delta := math.Abs(a - b)
				if delta > threshold {
					diverged = append(diverged, div{ID: id, Name: name, A: a, B: b, Delta: delta})
				}
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			if limit > 0 && len(diverged) > limit {
				diverged = diverged[:limit]
			}
			report := map[string]any{
				"object":         object,
				"field_a":        fieldA,
				"field_b":        fieldB,
				"threshold":      threshold,
				"scanned":        scanned,
				"with_both":      withBoth,
				"diverged_count": len(diverged),
				"diverged":       diverged,
			}
			blob, _ := json.MarshalIndent(report, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")
	cmd.Flags().StringVar(&object, "object", "", "Object api_name (e.g. company)")
	cmd.Flags().StringArrayVar(&fields, "field", nil, "First two --field args are compared (numeric attributes)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "Absolute-difference threshold (rows with ABS(a-b) > threshold are flagged)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Cap on returned diverged rows (0 = unlimited)")
	return cmd
}

func numericOf(v any) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case []byte:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}
