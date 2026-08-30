// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: top-level search command — hub marketplace search with local store caching.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var term string
	var category string
	var limit int

	cmd := &cobra.Command{
		Use:         "search",
		Short:       "Search the RapidAPI marketplace by term, category, and tags",
		Long:        "Search the RapidAPI hub marketplace for APIs by term (and optional category). Results are cached in the local store for offline re-querying.",
		Example:     "  rapidapi-pp-cli search --term weather --category Weather --limit 10\n  rapidapi-pp-cli search weather",
		Annotations: map[string]string{"pp:endpoint": "search.apis", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && term == "" {
				term = args[0]
			}
			where := map[string]any{"term": term}
			if category != "" {
				where["categoryNames"] = []string{category}
			}
			variables := map[string]any{
				"searchApiWhereInput":   where,
				"paginationInput":       map[string]any{"first": limit},
				"searchApiOrderByInput": map[string]any{"sortingFields": []map[string]any{{"by": "ASC", "fieldName": "ByRelevance"}}},
			}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "searchApis", variables, gqlResponsePaths["searchApis"])
			if err != nil {
				return err
			}
			// Cache results locally for offline re-query (store-backed search).
			if !flags.dryRun {
				if s, err := store.OpenWithContext(cmd.Context(), learnDBPath("")); err == nil {
					var items []map[string]any
					if json.Unmarshal(data, &items) == nil {
						for _, it := range items {
							_ = s.UpsertApis(mustJSON(it))
						}
					}
					_ = s.Close()
				}
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true, "title": true, "description": true, "slugifiedName": true, "pricing": true, "updatedAt": true, "categoryName": true, "score": true})
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Search term (also first positional arg)")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category name")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// cacheGenericRows upserts a JSON array of records into the store under the
// given resource type. Returns the number of records cached.
func cacheGenericRows(cmd *cobra.Command, s *store.Store, resource string, data json.RawMessage) int {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return 0
	}
	count := 0
	for _, it := range items {
		id, _ := it["id"].(string)
		if id == "" {
			continue
		}
		if err := s.Upsert(resource, id, mustJSON(it)); err == nil {
			count++
		}
	}
	return count
}
