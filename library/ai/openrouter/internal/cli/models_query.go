// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — DSL parser compiling key=val/key<val/key>=val expressions to SQL over local catalog.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/store"

	"github.com/spf13/cobra"
)

// expression: space-separated tokens like  tools=true cost.completion<1 ctx>=64k modality=text provider=anthropic
// Unstructured terms (no operator) are FTS5 lookups.
var queryTokenRe = regexp.MustCompile(`^([a-zA-Z0-9_.]+)\s*(<=|>=|!=|=|<|>)\s*(.+)$`)

func parseSizeSuffix(v string) (float64, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	mult := 1.0
	switch {
	case strings.HasSuffix(v, "k"):
		mult = 1_000
		v = strings.TrimSuffix(v, "k")
	case strings.HasSuffix(v, "m"):
		mult = 1_000_000
		v = strings.TrimSuffix(v, "m")
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}

// jsonPathExpr returns a SQL expression extracting the given dotted path
// from the `data` JSON column, cast appropriately.
func jsonPathExpr(path string) string {
	parts := strings.Split(path, ".")
	jp := "$"
	for _, p := range parts {
		jp += "." + p
	}
	return fmt.Sprintf("json_extract(data, '%s')", jp)
}

func newModelsQueryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var llm bool

	cmd := &cobra.Command{
		Use:         "query <expression>",
		Short:       "Query local model catalog with structured filters compiled to SQL",
		Example:     "  openrouter-pp-cli models query \"tools=true cost.completion<1 ctx>=64k\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expr := strings.Join(args, " ")
			tokens := strings.Fields(expr)

			where := []string{}
			params := []any{}
			ftsTerms := []string{}

			for _, tok := range tokens {
				if strings.HasPrefix(tok, "__") || strings.Contains(tok, "__printing_press_") {
					return usageErr(fmt.Errorf("invalid query token %q (use key=value, key<value, key>=value, or a search term)", tok))
				}
				m := queryTokenRe.FindStringSubmatch(tok)
				if m == nil {
					ftsTerms = append(ftsTerms, tok)
					continue
				}
				key, op, val := m[1], m[2], m[3]

				switch key {
				case "tools":
					// supported_parameters lives per-endpoint, but model.architecture or per-model.
					// We use FTS over data JSON for tools=true.
					if val == "true" {
						where = append(where, "data LIKE ?")
						params = append(params, "%\"tools\"%")
					}
				case "ctx", "context_length":
					n, err := parseSizeSuffix(val)
					if err != nil {
						return usageErr(fmt.Errorf("bad ctx value %q: %w", val, err))
					}
					where = append(where, fmt.Sprintf("context_length %s ?", op))
					params = append(params, int(n))
				case "cost.completion", "cost.prompt", "pricing.completion", "pricing.prompt":
					field := key
					if strings.HasPrefix(key, "cost.") {
						field = "pricing." + strings.TrimPrefix(key, "cost.")
					}
					n, err := strconv.ParseFloat(val, 64)
					if err != nil {
						return usageErr(fmt.Errorf("bad numeric for %s: %w", key, err))
					}
					where = append(where, fmt.Sprintf("CAST(%s AS REAL) %s ?", jsonPathExpr(field), op))
					params = append(params, n)
				case "modality":
					where = append(where, fmt.Sprintf("%s LIKE ?", jsonPathExpr("architecture.modality")))
					params = append(params, "%"+val+"%")
				case "provider":
					where = append(where, "id LIKE ?")
					params = append(params, val+"/%")
				case "id":
					where = append(where, fmt.Sprintf("id %s ?", op))
					params = append(params, val)
				case "name":
					where = append(where, fmt.Sprintf("name %s ?", op))
					params = append(params, val)
				default:
					// fallback: treat as JSON path comparison
					where = append(where, fmt.Sprintf("%s %s ?", jsonPathExpr(key), op))
					params = append(params, val)
				}
			}

			dbPath := defaultDBPath("openrouter-pp-cli")
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("open store: %w", err))
			}
			defer db.Close()

			// FTS5 fallback: filter rowids first, then intersect.
			var sqlStr string
			if len(ftsTerms) > 0 {
				where = append(where, "rowid IN (SELECT rowid FROM models_fts WHERE models_fts MATCH ?)")
				params = append(params, strings.Join(ftsTerms, " "))
			}
			sqlStr = "SELECT id, data FROM models"
			if len(where) > 0 {
				sqlStr += " WHERE " + strings.Join(where, " AND ")
			}
			sqlStr += " LIMIT ?"
			params = append(params, limit)

			rows, err := db.DB().QueryContext(cmd.Context(), sqlStr, params...)
			if err != nil {
				return apiErr(fmt.Errorf("query: %w", err))
			}
			defer rows.Close()

			results := []map[string]any{}
			for rows.Next() {
				var id string
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if json.Unmarshal(raw, &obj) != nil {
					obj = map[string]any{}
				}
				if _, ok := obj["id"]; !ok {
					obj["id"] = id
				}
				results = append(results, obj)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if llm {
				if len(results) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no matches")
					return nil
				}
				for _, r := range results {
					id, _ := r["id"].(string)
					ctx := ""
					if v, ok := r["context_length"]; ok {
						ctx = fmt.Sprintf("%v", v)
					}
					cc := ""
					if pricing, ok := r["pricing"].(map[string]any); ok {
						if v, ok := pricing["completion"]; ok {
							cc = fmt.Sprintf("%v", v)
						}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "id=%s ctx=%s cost.completion=%s\n", id, ctx, cc)
				}
				return nil
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			rows2 := make([][]string, 0, len(results))
			for _, r := range results {
				id, _ := r["id"].(string)
				ctx := ""
				if v, ok := r["context_length"]; ok {
					ctx = fmt.Sprintf("%v", v)
				}
				rows2 = append(rows2, []string{id, ctx})
			}
			return flags.printTable(cmd, []string{"ID", "CTX"}, rows2)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return")
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	return cmd
}
