// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopping/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopping/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopping/internal/store"
	"github.com/spf13/cobra"
)

// transientAPIError reports whether err is a retryable upstream hiccup: any
// 5xx, or a 400 whose body signals a transient search-shard failure ("all
// shards failed"). LemmeBuyIt's search backend intermittently returns the
// latter under load, so a single shard hiccup should not fail an index run.
func transientAPIError(err error) bool {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode >= 500 {
		return true
	}
	return apiErr.StatusCode == 400 && strings.Contains(strings.ToLower(apiErr.Body), "shards failed")
}

// getWithRetry wraps client.Get with a bounded retry on transient upstream
// errors. The backoff stays small so it fits inside the dogfood per-command
// timeout; non-transient errors return immediately.
func getWithRetry(ctx context.Context, c *client.Client, path string, params map[string]string) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := c.Get(ctx, path, params)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		if !transientAPIError(err) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(300*(attempt+1)) * time.Millisecond):
		}
	}
	return nil, lastErr
}

// pp:data-source live

// indexSummary is the JSON envelope `index` emits.
type indexSummary struct {
	Retailers       []string `json:"retailers"`
	ProductsIndexed int      `json:"products_indexed"`
	PricePoints     int      `json:"price_points"`
}

func newNovelIndexCmd(flags *rootFlags) *cobra.Command {
	var flagRetailer []string
	var flagShopping bool
	var flagOnSale bool
	var flagSearch string
	var flagBrand string
	var flagCategory string
	var flagMinDiscount float64
	var flagLimit int
	var flagMaxPages int
	var flagPriceHistory bool
	var flagDB string

	cmd := &cobra.Command{
		Use:     "index",
		Short:   "Pull products (and optional price history) from retailers into the local store",
		Example: "  shopping-pp-cli index --retailer walmart --retailer target --on-sale --limit 500",
		// No mcp:read-only: index writes the local store (products / price_points),
		// so the MCP tool defaults to could-write and the host prompts before use.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				which := "all accessible retailers"
				if len(flagRetailer) > 0 {
					which = fmt.Sprintf("retailers %v", flagRetailer)
				}
				surface := "/products"
				if flagShopping {
					surface = "/shopping/products"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would index %s via %s into the local store\n", which, surface)
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := openShopStore(ctx, resolveShopDBPath(flagDB))
			if err != nil {
				return err
			}
			defer db.Close()

			limit := flagLimit
			if limit <= 0 {
				limit = 200
			}
			maxPages := flagMaxPages
			if maxPages <= 0 {
				maxPages = 5
			}
			// Bound work under the dogfood matrix's flat per-command timeout.
			if cliutil.IsDogfoodEnv() {
				maxPages = 1
				if limit > 50 {
					limit = 50
				}
			}

			retailers := flagRetailer
			if len(retailers) == 0 {
				retailers, err = listRetailerIDs(ctx, c)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if cliutil.IsDogfoodEnv() && len(retailers) > 2 {
					retailers = retailers[:2]
				}
			}

			pageSize := limit
			if pageSize > 100 {
				pageSize = 100
			}

			baseParams := map[string]string{}
			if flagOnSale {
				baseParams["on_sale"] = "true"
			}
			if flagSearch != "" {
				baseParams["search_query"] = flagSearch
			}
			if flagBrand != "" {
				baseParams["brand"] = flagBrand
			}
			if flagCategory != "" {
				baseParams["category"] = flagCategory
			}
			if cmd.Flags().Changed("min-discount") {
				baseParams["min_discount_percent"] = strconv.FormatFloat(flagMinDiscount, 'f', -1, 64)
			}
			baseParams["limit"] = strconv.Itoa(pageSize)

			totalProducts := 0
			totalPoints := 0

			for _, rid := range retailers {
				path := "/retailers/" + rid + "/products"
				if flagShopping {
					path = "/retailers/" + rid + "/shopping/products"
				}

				stored := 0
				cursor := ""
				for page := 0; page < maxPages && stored < limit; page++ {
					params := map[string]string{}
					for k, v := range baseParams {
						params[k] = v
					}
					if cursor != "" {
						params["after"] = cursor
					}

					raw, err := getWithRetry(ctx, c, path, params)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					items, next := parseProductPage(raw)
					if len(items) == 0 {
						break
					}

					for _, item := range items {
						if stored >= limit {
							break
						}
						obj, err := store.DecodeJSONObject(item)
						if err != nil {
							continue
						}
						// The live API returns retailer_id (singular, often
						// null). The store's NOT NULL retailers_id column reads
						// the retailers_id key, so set it (and parent_id) to the
						// retailer we fetched before re-marshaling.
						obj["retailers_id"] = rid
						obj["parent_id"] = rid
						remarshaled, err := json.Marshal(obj)
						if err != nil {
							continue
						}
						if flagShopping {
							if err := db.UpsertShopping(remarshaled); err != nil {
								return fmt.Errorf("upsert shopping (%s): %w", rid, err)
							}
						} else {
							if err := db.UpsertProducts(remarshaled); err != nil {
								return fmt.Errorf("upsert products (%s): %w", rid, err)
							}
						}
						stored++

						if flagPriceHistory && !flagShopping && !cliutil.IsDogfoodEnv() && stored <= 25 {
							// Fetch by the merchant SKU (the price-history path key),
							// but store price_points.product_id as the product's `id`
							// so it joins products.id in watch-status/price-drops.
							sku := productSKU(obj)
							pid := store.ResourceIDString(obj["id"])
							if sku != "" && pid != "" {
								n, err := indexPriceHistory(ctx, c, db, rid, sku, pid)
								if err != nil {
									return classifyAPIError(err, flags)
								}
								totalPoints += n
							}
						}
					}

					if next == "" || next == cursor {
						break
					}
					cursor = next
				}
				totalProducts += stored
			}

			summary := indexSummary{
				Retailers:       retailers,
				ProductsIndexed: totalProducts,
				PricePoints:     totalPoints,
			}
			if summary.Retailers == nil {
				summary.Retailers = []string{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}
	cmd.Flags().StringSliceVar(&flagRetailer, "retailer", nil, "Retailer IDs to index (repeatable); defaults to all accessible retailers")
	cmd.Flags().BoolVar(&flagShopping, "shopping", false, "Use the free /shopping/products surface instead of /products")
	cmd.Flags().BoolVar(&flagOnSale, "on-sale", false, "Only index products currently on sale")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Search query passed to the API")
	cmd.Flags().StringVar(&flagBrand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter by category")
	cmd.Flags().Float64Var(&flagMinDiscount, "min-discount", 0, "Minimum discount percentage")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Maximum products to store per retailer")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 5, "Maximum pages to fetch per retailer")
	cmd.Flags().BoolVar(&flagPriceHistory, "price-history", false, "Also fetch and store weekly price history (non-shopping only)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local store (defaults to SHOPPING_DB or the share path)")
	return cmd
}

// listRetailerIDs calls GET /retailers and returns each retailer's id.
func listRetailerIDs(ctx context.Context, c *client.Client) ([]string, error) {
	raw, err := c.Get(ctx, "/retailers", nil)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parse retailers list: %w", err)
	}
	ids := make([]string, 0, len(arr))
	for _, r := range arr {
		if id, ok := r["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// parseProductPage extracts the product objects and next cursor from a
// product-list response of shape {data:[...], page_info:{next_cursor}}.
func parseProductPage(raw json.RawMessage) ([]json.RawMessage, string) {
	var envelope struct {
		Data     []json.RawMessage `json:"data"`
		PageInfo struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"page_info"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		// A bare array is a tolerated fallback shape.
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			return arr, ""
		}
		return nil, ""
	}
	next := ""
	if envelope.PageInfo.NextCursor != nil {
		next = *envelope.PageInfo.NextCursor
	} else if envelope.NextCursor != nil {
		next = *envelope.NextCursor
	}
	return envelope.Data, next
}

// productSKU returns the identifier used to fetch a product's price history:
// the unique_merchant_sku when present, else the numeric id.
func productSKU(obj map[string]any) string {
	if v, ok := obj["unique_merchant_sku"]; ok {
		if s := store.ResourceIDString(v); s != "" {
			return s
		}
	}
	if v, ok := obj["id"]; ok {
		return store.ResourceIDString(v)
	}
	return ""
}

// indexPriceHistory fetches one product's price-history series and inserts
// each point into price_points. Returns the number of points written.
func indexPriceHistory(ctx context.Context, c *client.Client, db *store.Store, rid, sku, productID string) (int, error) {
	path := "/retailers/" + rid + "/products/" + sku + "/price-history"
	raw, err := getWithRetry(ctx, c, path, nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Series []struct {
			TS           string   `json:"ts"`
			AmzBuyBox    *float64 `json:"amz_buy_box"`
			WalmartPrice *float64 `json:"walmart_price"`
			AmzRetail    *float64 `json:"amz_retail"`
		} `json:"series"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("parse price history: %w", err)
	}
	written := 0
	for _, p := range resp.Series {
		if p.TS == "" {
			continue
		}
		price := coalesceFloat(p.AmzBuyBox, p.WalmartPrice, p.AmzRetail)
		if _, err := db.DB().ExecContext(ctx,
			`INSERT OR REPLACE INTO price_points
			 (retailers_id, product_id, ts, price, amz_buy_box, walmart_price, source)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			rid, productID, p.TS, nullArg(price), nullArg(p.AmzBuyBox), nullArg(p.WalmartPrice), "price-history",
		); err != nil {
			return written, fmt.Errorf("insert price point: %w", err)
		}
		written++
	}
	return written, nil
}

// coalesceFloat returns the first non-nil value, or nil when all are nil.
func coalesceFloat(vals ...*float64) *float64 {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// nullArg maps a *float64 to a driver argument: nil becomes SQL NULL.
func nullArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
