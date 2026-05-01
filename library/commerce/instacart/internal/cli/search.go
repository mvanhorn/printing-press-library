package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
)

// SearchResult is a thin re-export so callers in this package can keep
// referring to a familiar type even though the real definition lives in
// internal/gql.
type SearchResult = gql.SearchResult

// newSearchCmd implements `instacart search <query> --store <slug>`. It
// resolves products by chaining ShopCollectionScoped -> Autosuggestions ->
// Items, all via direct GraphQL with the existing session cookies. No
// browser, no HTML scraping, no Playwright.
func newSearchCmd() *cobra.Command {
	var storeFlag string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query...>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Search products at a retailer by natural language",
		Long: `Search for products at a specific retailer. Uses your saved session to
bootstrap a retailer inventory token (cached 6h), feeds the query through
Instacart's autosuggest GraphQL API, extracts productIds from suggestion
tracking URLs, and fetches full product data via the Items API.

Three direct GraphQL round trips, all under 1 second warm.

Exit codes: 0 success, 3 auth missing, 4 no results, 7 network/api error.`,
		Example: `  instacart search "milk" --store costco
  instacart search "2% milk" --store costco --limit 3
  instacart search "olive oil" --store costco --json`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if storeFlag == "" {
				return coded(ExitUsage, "--store is required (e.g. --store costco)")
			}
			query := strings.Join(args, " ")
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			results, err := gql.ResolveProducts(app.Ctx, app.Session, app.Cfg, app.Store, storeFlag, query, limit)
			if err != nil {
				return coded(ExitNotFound, "%v", err)
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no results for %q at %s\n", query, storeFlag)
				return coded(ExitNotFound, "no results")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "results for %q at %s:\n", query, storeFlag)
			for i, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %s\n      item_id=%s\n", i+1, r.Name, r.ItemID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&storeFlag, "store", "", "Retailer slug (e.g. costco) - required")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results to return")
	return cmd
}
