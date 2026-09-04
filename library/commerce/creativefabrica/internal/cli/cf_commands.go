// Hand-authored absorbed catalog commands: free, pod, designer, product,
// categories, types, auth.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/algolia"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newFreeCmd(flags *rootFlags) *cobra.Command {
	var itemType, category, sortBy string
	var limit int
	cmd := &cobra.Command{
		Use:         "free [query]",
		Short:       "List free assets (newest first), optionally filtered",
		Example:     strings.Trim("\n  creativefabrica-pp-cli free --type Fonts --limit 20\n  creativefabrica-pp-cli free \"christmas\" --agent", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := catalogQuery{free: true, itemType: itemType, category: category, sortBy: sortBy, limit: limit}
			if len(args) > 0 {
				q.query = args[0]
			}
			return runCatalogSearch(cmd, flags, q)
		},
	}
	cmd.Flags().StringVar(&itemType, "type", "", "Product type filter")
	cmd.Flags().StringVar(&category, "category", "", "Category filter")
	cmd.Flags().StringVar(&sortBy, "sort", "newest", "Sort order: relevance | newest")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

// pp:data-source live
func newPodCmd(flags *rootFlags) *cobra.Command {
	var itemType, category, sortBy string
	var free bool
	var maxPrice float64
	var limit int
	cmd := &cobra.Command{
		Use:         "pod [query]",
		Short:       "List print-on-demand / commercial-license assets",
		Long:        "List assets cleared for print-on-demand (commercial use). Combine with --free or --max-price for sourcing. For full filter control use 'find --pod'.",
		Example:     strings.Trim("\n  creativefabrica-pp-cli pod \"t-shirt\" --max-price 3 --csv\n  creativefabrica-pp-cli pod --type Graphics --free --agent", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := catalogQuery{pod: true, free: free, itemType: itemType, category: category, sortBy: sortBy, maxPrice: maxPrice, limit: limit}
			if len(args) > 0 {
				q.query = args[0]
			}
			return runCatalogSearch(cmd, flags, q)
		},
	}
	cmd.Flags().StringVar(&itemType, "type", "", "Product type filter")
	cmd.Flags().StringVar(&category, "category", "", "Category filter")
	cmd.Flags().StringVar(&sortBy, "sort", "relevance", "Sort order: relevance | newest")
	cmd.Flags().BoolVar(&free, "free", false, "Only free POD assets")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

// pp:data-source live
func newDesignerCmd(flags *rootFlags) *cobra.Command {
	var sortBy string
	var limit int
	cmd := &cobra.Command{
		Use:         "designer <id|name>",
		Short:       "Browse a designer's catalog",
		Long:        "List a designer's products by numeric id or exact name. For an aggregate profile use 'designer-stats'; to compare two designers use 'designer-compare'.",
		Example:     strings.Trim("\n  creativefabrica-pp-cli designer 2880714 --limit 25\n  creativefabrica-pp-cli designer \"DigiArt\" --sort newest --agent", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("designer id or name is required"))
			}
			q := catalogQuery{designer: args[0], sortBy: sortBy, limit: limit}
			return runCatalogSearch(cmd, flags, q)
		},
	}
	cmd.Flags().StringVar(&sortBy, "sort", "newest", "Sort order: relevance | newest")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	return cmd
}

// pp:data-source live
func newProductCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "product <objectID>",
		Short:       "Show a single product's catalog metadata by object id",
		Example:     strings.Trim("\n  creativefabrica-pp-cli product 21415690 --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("product objectID is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newAlgoliaClient(flags)
			results, err := c.Search(ctx, algolia.SearchRequest{
				IndexName:   algolia.IndexRelevance,
				Filters:     fmt.Sprintf("objectID:%s", quoteFacet(args[0])),
				HitsPerPage: 1,
			})
			if err != nil {
				return apiErr(err)
			}
			if len(results) == 0 || len(results[0].Hits) == 0 {
				return notFoundErr(fmt.Errorf("no product with objectID %q", args[0]))
			}
			return flags.printJSON(cmd, toView(results[0].Hits[0]))
		},
	}
	return cmd
}

type facetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// pp:data-source live
func newFacetCmd(flags *rootFlags, facet, use, short string) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) > 0 {
				query = args[0]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newAlgoliaClient(flags)
			results, err := c.Search(ctx, algolia.SearchRequest{
				IndexName: algolia.IndexRelevance, Query: query, HitsPerPage: 0,
				Facets: []string{facet}, MaxValuesPerFacet: 1000,
			})
			if err != nil {
				return apiErr(err)
			}
			var counts []facetCount
			if len(results) > 0 {
				for v, n := range results[0].Facets[facet] {
					counts = append(counts, facetCount{Value: v, Count: n})
				}
			}
			sort.Slice(counts, func(i, j int) bool {
				if counts[i].Count != counts[j].Count {
					return counts[i].Count > counts[j].Count
				}
				return counts[i].Value < counts[j].Value
			})
			if limit > 0 && len(counts) > limit {
				counts = counts[:limit]
			}
			if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, counts)
			}
			rows := make([][]string, 0, len(counts))
			for _, fc := range counts {
				rows = append(rows, []string{fc.Value, fmt.Sprintf("%d", fc.Count)})
			}
			return flags.printTable(cmd, []string{strings.ToUpper(facet), "COUNT"}, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max values to return (0 = all)")
	return cmd
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	c := newFacetCmd(flags, "category", "categories [query]", "List catalog categories with counts (optionally scoped to a query)")
	c.Example = strings.Trim("\n  creativefabrica-pp-cli categories --limit 40\n  creativefabrica-pp-cli categories \"halloween\" --agent", "\n")
	return c
}

func newTypesCmd(flags *rootFlags) *cobra.Command {
	c := newFacetCmd(flags, "type", "types [query]", "List product types with counts")
	c.Example = strings.Trim("\n  creativefabrica-pp-cli types\n  creativefabrica-pp-cli types \"svg\" --json", "\n")
	return c
}

// pp:data-source live
func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the public catalog search key (set-key, status)",
		Long: `The Creative Fabrica catalog uses a public, search-only Algolia key (the same
one the website ships in its JavaScript). The CLI resolves it from the
CREATIVEFABRICA_ALGOLIA_API_KEY env var, a local cache, or best-effort
auto-discovery. Use 'auth set-key' to cache it manually.`,
	}
	setKey := &cobra.Command{
		Use:   "set-key <key>",
		Short: "Cache the public catalog search key locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search key is required"))
			}
			if err := algolia.SaveCreds(algolia.DefaultAppID, strings.TrimSpace(args[0])); err != nil {
				return apiErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved catalog key to %s\n", algolia.CredsPath())
			return nil
		},
	}
	status := &cobra.Command{
		Use:         "status",
		Short:       "Show whether a catalog key is configured",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{
				"key_configured": keyConfigured(),
				"app_id":         algolia.DefaultAppID,
				"cache_path":     algolia.CredsPath(),
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, report)
			}
			if keyConfigured() {
				fmt.Fprintln(cmd.OutOrStdout(), "catalog key: configured")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "catalog key: not configured (will attempt auto-discovery; set CREATIVEFABRICA_ALGOLIA_API_KEY or run 'auth set-key')")
			}
			return nil
		},
	}
	cmd.AddCommand(setKey, status)
	return cmd
}
