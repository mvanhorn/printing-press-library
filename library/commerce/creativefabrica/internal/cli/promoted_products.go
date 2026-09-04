// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/spf13/cobra"
)

func newProductsPromotedCmd(flags *rootFlags) *cobra.Command {
	var bodyRequests string

	cmd := &cobra.Command{
		Use:         "products",
		Short:       "Low-level Algolia multi-query passthrough (prefer the top-level search/browse/free/pod commands)",
		Long:        "Low-level Algolia multi-query passthrough (prefer the top-level search/browse/free/pod commands). Queries go through the credential-aware catalog client, not the generated HTTP client.",
		Example:     "  creativefabrica-pp-cli products\n  creativefabrica-pp-cli products --requests '[{\"indexName\":\"prod_Productsv2\",\"query\":\"svg\"}]'",
		Annotations: map[string]string{"pp:endpoint": "products.search", "pp:method": "POST", "pp:path": "/1/indexes/*/queries", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search catalog via Algolia multi-query\n")
				return nil
			}
			reqs, err := ParseAlgoliaRequests(bodyRequests)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			results, err := newAlgoliaClient(flags).Search(ctx, reqs...)
			if err != nil {
				return apiErr(err)
			}
			data, err := json.Marshal(map[string]any{"results": results})
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				for _, r := range results {
					for _, h := range r.Hits {
						items = append(items, map[string]any{
							"objectID": h.ObjectID,
							"name":     h.NameEN,
							"type":     h.Type,
							"price":    h.Price.Float(),
						})
					}
				}
				if len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyRequests, "requests", "", "JSON array of Algolia query objects (indexName, query, page, hitsPerPage, filters)")

	return cmd
}

func ParseAlgoliaRequests(raw string) ([]algolia.SearchRequest, error) {
	if strings.TrimSpace(raw) == "" {
		return []algolia.SearchRequest{{
			IndexName:   algolia.IndexRelevance,
			HitsPerPage: 20,
		}}, nil
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parsing --requests JSON: %w", err)
	}
	var rows []any
	switch v := payload.(type) {
	case []any:
		rows = v
	case map[string]any:
		inner, ok := v["requests"].([]any)
		if !ok {
			return nil, fmt.Errorf("parsing --requests JSON: object must contain a requests array")
		}
		rows = inner
	default:
		return nil, fmt.Errorf("parsing --requests JSON: expected array or {\"requests\":[...]}")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("parsing --requests JSON: requests array is empty")
	}
	out := make([]algolia.SearchRequest, 0, len(rows))
	for i, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parsing --requests JSON: request %d is not an object", i)
		}
		req := algolia.SearchRequest{IndexName: algolia.IndexRelevance}
		if s, _ := m["indexName"].(string); s != "" {
			req.IndexName = s
		}
		if s, _ := m["query"].(string); s != "" {
			req.Query = s
		}
		if s, _ := m["filters"].(string); s != "" {
			req.Filters = s
		}
		req.Page = jsonInt(m["page"])
		if n := jsonInt(m["hitsPerPage"]); n > 0 {
			req.HitsPerPage = n
		}
		if facets, ok := m["facets"].([]any); ok {
			for _, f := range facets {
				if s, ok := f.(string); ok && s != "" {
					req.Facets = append(req.Facets, s)
				}
			}
		}
		out = append(out, req)
	}
	return out, nil
}

func jsonInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
