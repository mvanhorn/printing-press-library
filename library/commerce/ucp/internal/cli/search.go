package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/registry"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/transport"
	"github.com/mvanhorn/printing-press-library/library/commerce/ucp/internal/ucp"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var merchant string
	var limit int
	var allPet bool
	var transportFlag string
	var profileURL string

	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search a UCP merchant's catalog",
		Example: `  ucp-pp-cli search "rope toy" --merchant bark.co --limit 5 --json`,
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			ctx := cmd.Context()

			// --all-pet: fan out across Grade-A pet merchants with catalog search.
			// Grade B merchants (kongcompany.com) are excluded — no catalog endpoint to fan into.
			if allPet {
				var petDomains []string
				for _, m := range registry.Default() {
					if m.HasRopeToys && m.Grade == "A" {
						petDomains = append(petDomains, m.Domain)
					}
				}
				// Fan out concurrently across pet merchants using FanoutRun.
				results, errs := cliutil.FanoutRun(
					ctx,
					petDomains,
					func(d string) string { return d },
					func(ctx context.Context, domain string) ([]ucp.SearchHit, error) {
						return transport.ShopifyProductsSearch(ctx, domain, query, limit)
					},
				)
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)
				// Initialize as empty slice (not nil) so json.Encode emits [] not null
				// when no merchants return hits.
				allHits := []ucp.SearchHit{}
				for _, r := range results {
					allHits = append(allHits, r.Value...)
				}
				return printSearchHits(cmd, flags, allHits)
			}

			if merchant == "" {
				return usageErr(fmt.Errorf("--merchant is required (or use --all-pet to search all pet stores)"))
			}

			// For localhost/127.0.0.1 targets (CI fixture), always use the REST client.
			if strings.HasPrefix(merchant, "127.0.0.1") || strings.HasPrefix(merchant, "localhost") {
				c, cerr := ucp.NewMerchantClient(ctx, merchant)
				if cerr != nil {
					return fmt.Errorf("connect to merchant %s: %w", merchant, cerr)
				}
				hits, err := c.Search(ctx, query, limit)
				if err != nil {
					return fmt.Errorf("search: %w", err)
				}
				return printSearchHits(cmd, flags, hits)
			}

			// Etsy adapter (non-UCP merchant).
			if merchant == "etsy.com" || merchant == "www.etsy.com" {
				hits, err := transport.EtsySearch(ctx, query, limit)
				if err != nil {
					return err
				}
				return printSearchHits(cmd, flags, hits)
			}

			// eBay adapter (non-UCP merchant).
			if merchant == "ebay.com" || merchant == "www.ebay.com" {
				hits, err := transport.EbaySearch(ctx, query, limit)
				if err != nil {
					return err
				}
				return printSearchHits(cmd, flags, hits)
			}

			// --transport mcp: fetch manifest and call McpSearch directly.
			if transportFlag == "mcp" {
				m, err := ucp.FetchManifest(ctx, merchant)
				if err != nil {
					return fmt.Errorf("fetch manifest for MCP transport: %w", err)
				}
				hits, err := transport.McpSearch(ctx, m, query, limit, profileURL)
				if err != nil {
					return fmt.Errorf("MCP catalog search: %w", err)
				}
				return printSearchHits(cmd, flags, hits)
			}

			// --transport products-json: legacy Shopify products.json path.
			if transportFlag == "products-json" {
				hits, err := transport.ShopifyProductsSearch(ctx, merchant, query, limit)
				if err != nil {
					return fmt.Errorf("shopify catalog search: %w", err)
				}
				return printSearchHits(cmd, flags, hits)
			}

			// --transport auto (default): try products.json first; fall through to MCP
			// when products.json 500s or returns zero hits and manifest declares mcp transport.
			hits, err := transport.ShopifyProductsSearch(ctx, merchant, query, limit)
			if err != nil || len(hits) == 0 {
				// Attempt MCP fallthrough: fetch manifest to check if MCP is available.
				m, merr := ucp.FetchManifest(ctx, merchant)
				if merr == nil && hasMCPTransport(m) {
					mcpHits, mcpErr := transport.McpSearch(ctx, m, query, limit, profileURL)
					if mcpErr == nil {
						return printSearchHits(cmd, flags, mcpHits)
					}
					// MCP also failed — return original products.json error if we had one,
					// or surface the MCP error when products.json was empty-but-clean.
					if err != nil {
						return fmt.Errorf("shopify catalog search: %w (MCP fallback also failed: %v)", err, mcpErr)
					}
					return fmt.Errorf("products.json returned no hits and MCP fallback failed: %w", mcpErr)
				}
			}
			if err != nil {
				return fmt.Errorf("shopify catalog search: %w", err)
			}
			return printSearchHits(cmd, flags, hits)
		},
	}

	cmd.Flags().StringVar(&merchant, "merchant", "", "Merchant domain (e.g. bark.co)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results to return")
	cmd.Flags().BoolVar(&allPet, "all-pet", false, "Fan out query across all rope-toy pet merchants (bark.co, ruffwear.com, sitstay.com)")
	cmd.Flags().StringVar(&transportFlag, "transport", "auto", "Transport: auto (default — try products.json then MCP), products-json, or mcp")
	cmd.Flags().StringVar(&profileURL, "profile-url", "", "UCP agent profile URL (default: https://www.igvita.com/ucp/profile.json)")
	return cmd
}

// hasMCPTransport returns true if the manifest declares any mcp-transport service with an endpoint.
func hasMCPTransport(m *ucp.Manifest) bool {
	for _, svcs := range m.UCP.Services {
		for _, s := range svcs {
			if s.Transport == "mcp" && s.Endpoint != "" {
				return true
			}
		}
	}
	return false
}

func printSearchHits(cmd *cobra.Command, flags *rootFlags, hits []ucp.SearchHit) error {
	if flags.asJSON {
		// Always emit [] (not null) for empty/nil results so consumers can
		// `jq '. | length'` without special-casing the empty response.
		if hits == nil {
			hits = []ucp.SearchHit{}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}

	if len(hits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results.")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "TITLE\tPRICE\tSKU\tURL")
	for _, h := range hits {
		priceStr := fmt.Sprintf("%d¢", h.Price)
		if h.Price > 0 {
			priceStr = fmt.Sprintf("$%.2f", float64(h.Price)/100)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Title, priceStr, h.SKU, h.URL)
	}
	return tw.Flush()
}
