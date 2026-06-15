// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command: `products get <itemId>` — full product detail from the
// PDP page's schema.org JSON-LD, with price enriched from the local store when
// the item was previously seen in a search.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/daraz/internal/cliutil"

	"github.com/spf13/cobra"
)

var darazHTTPClient = &http.Client{}

// darazPDPLimiter throttles direct PDP fetches so a scripted `products get`
// loop does not hammer Daraz's servers. The generated API client rate-limits
// its own calls; this covers the one hand-authored direct fetch.
var darazPDPLimiter = cliutil.NewAdaptiveLimiter(2)

func fetchPDPHTML(ctx context.Context, itemID string) (string, error) {
	url := fmt.Sprintf("https://www.daraz.pk/products/-i%s.html", itemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", darazUA)
	if darazPDPLimiter != nil {
		darazPDPLimiter.Wait()
	}
	resp, err := darazHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("product page returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func newProductsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <itemId>",
		Short:       "Show full product detail (brand, category, specs, description) for an item by ID.",
		Long:        "Show full product detail for an item by its numeric ID, parsed from the product page's structured data. Price is included when the item was previously captured by a search (deals/value/compare/watch/products).\n\nFor comparing many listings, use 'compare' or 'products' instead.",
		Example:     "  daraz-pp-cli products get 599201597 --agent",
		Annotations: map[string]string{"pp:method": "GET", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an item ID is required, e.g. products get 599201597"))
			}
			itemID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			html, err := fetchPDPHTML(ctx, itemID)
			if err != nil {
				return fmt.Errorf("fetching product %s: %w", itemID, err)
			}
			pd := extractProductDetail(html, itemID)
			if pd.Name == "" {
				return fmt.Errorf("no product found for item %s (check the ID)", itemID)
			}
			if s, e := openDarazStore(ctx, flags); e == nil {
				var price, orig sql.NullFloat64
				var last sql.NullInt64
				row := s.DB().QueryRowContext(ctx, `SELECT price, original_price, last_seen FROM daraz_products_seen WHERE item_id=?`, itemID)
				if row.Scan(&price, &orig, &last) == nil {
					pd.Price = price.Float64
					pd.OriginalPrice = orig.Float64
					if last.Int64 > 0 {
						pd.PriceAsOf = time.Unix(last.Int64, 0).Format("2006-01-02 15:04")
					}
				}
				_ = s.Close()
			}
			return emitDaraz(cmd, flags, pd)
		},
	}
	return cmd
}
