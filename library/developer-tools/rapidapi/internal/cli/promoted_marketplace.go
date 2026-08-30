// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic search command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMarketplacePromotedCmd(flags *rootFlags) *cobra.Command {
	var term string
	var category string
	var tags string
	var limit int
	var sort string
	var savedOnly bool

	cmd := &cobra.Command{
		Use:     "marketplace [term] [flags]",
		Short:   "Search APIs across the marketplace with filters, facets, scores, and pagination",
		Long:    "Search the RapidAPI marketplace. Results include popularity score, avg latency, service level, pricing, and owner.",
		Example: "  rapidapi-pp-cli marketplace search weather --category Weather --limit 5\n  rapidapi-pp-cli marketplace search --term=linkedin --tags=scraping --sort=ByUpdatedAt",
		Annotations: map[string]string{"pp:endpoint": "marketplace.search", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true", "pp:happy-args": "term=weather;--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare invocation prints help (or usage error for machine callers).
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			if len(args) > 0 && term == "" {
				term = args[0]
			}
			if term == "" && !cmd.Flags().Changed("term") && !flags.dryRun {
				return fmt.Errorf("search term required (positional or --term)")
			}

			where := map[string]any{"term": term}
			if category != "" {
				where["categoryNames"] = []string{category}
			}
			if tags != "" {
				where["tags"] = []string{tags}
			}
			if savedOnly {
				where["favoritesOnly"] = true
			}

			orderBy := map[string]any{"sortingFields": []map[string]any{{"by": "ASC", "fieldName": sort}}}
			pagination := map[string]any{"first": limit}

			variables := map[string]any{
				"searchApiWhereInput":    where,
				"paginationInput":        pagination,
				"searchApiOrderByInput":  orderBy,
			}

			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "searchApis", variables, gqlResponsePaths["searchApis"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true, "title": true, "description": true, "slugifiedName": true, "thumbnail": true, "pricing": true, "updatedAt": true, "categoryName": true, "isSavedApi": true, "visibility": true, "score": true, "user": true})
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Search term (also accepted as first positional arg)")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category name (e.g. Weather, Finance, AI)")
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by a single tag (comma-separated not supported by the hub gateway)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&sort, "sort", "ByRelevance", "Sort field: ByRelevance, ByAlphabetical, ByUpdatedAt, installsAllTime")
	cmd.Flags().BoolVar(&savedOnly, "saved", false, "Only show APIs you saved as favorites")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
