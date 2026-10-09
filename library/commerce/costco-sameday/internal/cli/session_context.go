// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored session context: global --zip, `session context`, and the
// live product search used by `search --data-source live`.
// Preserved across regenerate via registerNovelCommand / registerClientHook.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/cliutil"
	"github.com/spf13/cobra"
)

// sessionZipFlag backs the global --zip flag.
var sessionZipFlag string

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if root.PersistentFlags().Lookup("zip") == nil {
			root.PersistentFlags().StringVar(&sessionZipFlag, "zip", "", "Delivery ZIP used to look up the Costco shop, zone and slots (default: last --zip, then $"+client.SessionZipEnv+", then account location)")
		}
		for _, child := range root.Commands() {
			if child.Name() == "session" {
				addNovelCommandIfAbsent(child, newSessionContextCmd(flags))
			}
		}
	})
	registerClientHook(func(c *client.Client) error {
		if strings.TrimSpace(sessionZipFlag) != "" {
			if err := client.ValidatePostalCode(sessionZipFlag); err != nil {
				return usageErr(fmt.Errorf("invalid --zip: %w", err))
			}
		} else if env := os.Getenv(client.SessionZipEnv); strings.TrimSpace(env) != "" {
			// Only the selected source is validated: a valid --zip overrides
			// a bad environment value.
			if err := client.ValidatePostalCode(env); err != nil {
				return usageErr(fmt.Errorf("invalid %s: %w", client.SessionZipEnv, err))
			}
		}
		c.SetSessionPostalCode(sessionZipFlag)
		return nil
	})
}

func newSessionContextCmd(flags *rootFlags) *cobra.Command {
	var refresh bool
	var withCart bool
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Resolve the delivery ZIP, Costco shopId, zoneId and active cartId for this session",
		Long: strings.TrimSpace(`
Shows the context that cart, slots, retailer and product commands fill in
automatically when --shop-id / --cart-id / --zone-id / --postal-code /
--user-location are omitted. Explicit flags always win.

ZIP precedence: a postalCode or userLocation.postalCode on this command, then
--zip, then $COSTCO_SAMEDAY_ZIP, then the last explicit --zip (remembered in
the state dir), then the account/IP geolocation. The shop is looked up from
the ZIP. The account/IP zone is used only when its ZIP matches; a different
ZIP uses RetailersZone. A postal code on the command is not remembered as the
default. The cart needs a signed-in session; an explicit shop id resolves it
without a ZIP.`),
		Example: `  costco-sameday-pp-cli session context --zip 98027
  costco-sameday-pp-cli session context --json
  costco-sameday-pp-cli session context --refresh`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:narrative": "session.context"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.IsDryRun() || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "would": "GET /graphql Geolocation, ShopCollectionScoped, ActiveCartId"})
			}
			sc, resolveErr := c.ResolveSessionContext(cmd.Context(), client.SessionNeeds{Shop: true, Zone: true, Location: true, Cart: withCart, Refresh: refresh})
			out := map[string]any{
				"postal_code":        sc.PostalCode,
				"postal_code_source": sc.PostalCodeSource,
				"zone_id":            sc.ZoneID,
				"shop_id":            sc.ShopID,
				"retailer_location":  sc.RetailerLocationID,
				"cart_id":            sc.CartID,
			}
			if resolveErr != nil {
				out["error"] = resolveErr.Error()
			}
			if err := flags.printJSON(cmd, out); err != nil {
				return err
			}
			if resolveErr != nil {
				return classifyAPIErrorOnly(resolveErr)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Ignore cached shop/zone/cart values and look them up again")
	cmd.Flags().BoolVar(&withCart, "cart", true, "Also resolve the active cartId (needs a signed-in session)")
	return cmd
}

// searchItemsBatch mirrors the web app's Items hydration batch size.
const searchItemsBatch = 25

// liveProductSearch runs SearchResultsPlacements as a persisted-query POST
// (the server rejects ad-hoc query text) and hydrates grid item ids via Items.
func liveProductSearch(ctx context.Context, c *client.Client, query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	vars := map[string]any{
		"action": nil, "query": query, "pageViewId": newSearchPageViewID(),
		"elevatedProductId": nil, "searchSource": "search", "filters": []any{},
		"disableReformulation": false, "disableLlm": false, "forceInspiration": false,
		"orderBy": "bestMatch", "clusterId": nil, "includeDebugInfo": false,
		"clusteringStrategy": nil, "contentManagementSearchParams": map[string]any{"itemGridColumnCount": 4},
		"first": 4,
	}
	var sc client.SessionContext
	var resolveErr error
	if !c.IsDryRun() && !cliutil.IsVerifyEnv() {
		sc, resolveErr = c.ResolveSessionContext(ctx, client.SessionNeeds{Shop: true, Zone: true, Location: true})
		if resolveErr != nil {
			return nil, resolveErr
		}
		vars["shopId"], vars["zoneId"], vars["postalCode"] = sc.ShopID, sc.ZoneID, sc.PostalCode
	}
	body := map[string]any{"operationName": "SearchResultsPlacements", "variables": vars}
	data, _, err := c.PostQueryWithParams(ctx, "/graphql", nil, body)
	if err != nil {
		return nil, err
	}
	if c.IsDryRun() || cliutil.IsVerifyEnv() {
		return []json.RawMessage{data}, nil
	}
	hydrated, ids := searchGridItems(data)
	var out []json.RawMessage
	seen := map[string]bool{}
	add := func(item map[string]any) {
		id, _ := item["id"].(string)
		if id == "" || seen[id] || len(out) >= limit {
			return
		}
		seen[id] = true
		if b, err := json.Marshal(compactSearchItem(item)); err == nil {
			out = append(out, b)
		}
	}
	byID := map[string]map[string]any{}
	for _, it := range hydrated {
		if id, _ := it["id"].(string); id != "" {
			byID[id] = it
		}
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	var pending []string
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			pending = append(pending, id)
		}
	}
	for start := 0; start < len(pending); start += searchItemsBatch {
		end := start + searchItemsBatch
		if end > len(pending) {
			end = len(pending)
		}
		items, err := fetchSearchItems(ctx, c, pending[start:end], sc)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if id, _ := it["id"].(string); id != "" {
				byID[id] = it
			}
		}
	}
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			add(it)
		}
	}
	return out, nil
}

func newSearchPageViewID() string {
	return client.NewPageViewID()
}

// searchGridItems returns hydrated grid items and every grid item id in
// display order (ads and carousels are skipped).
func searchGridItems(data json.RawMessage) ([]map[string]any, []string) {
	var payload struct {
		Data struct {
			SearchResultsPlacements struct {
				Placements []struct {
					Content map[string]any `json:"content"`
				} `json:"placements"`
			} `json:"searchResultsPlacements"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil, nil
	}
	var items []map[string]any
	var ids []string
	seen := map[string]bool{}
	for _, p := range payload.Data.SearchResultsPlacements.Placements {
		if tn, _ := p.Content["__typename"].(string); tn != "SearchContentManagementSearchItemGrid" {
			continue
		}
		if raw, ok := p.Content["items"].([]any); ok {
			for _, x := range raw {
				if m, ok := x.(map[string]any); ok {
					items = append(items, m)
					if id, _ := m["id"].(string); id != "" && !seen[id] {
						seen[id] = true
						ids = append(ids, id)
					}
				}
			}
		}
		if raw, ok := p.Content["itemIds"].([]any); ok {
			for _, x := range raw {
				if id, _ := x.(string); id != "" && !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return items, ids
}

func fetchSearchItems(ctx context.Context, c *client.Client, ids []string, sc client.SessionContext) ([]map[string]any, error) {
	vars := map[string]any{"ids": ids, "shopId": sc.ShopID, "zoneId": sc.ZoneID, "postalCode": sc.PostalCode}
	vb, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/graphql", map[string]string{"operationName": "Items", "variables": string(vb)})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("Items: decoding response: %w", err)
	}
	return payload.Data.Items, nil
}

// compactSearchItem keeps the product fields an agent needs.
func compactSearchItem(item map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"id", "productId", "name", "brandName", "size"} {
		if v, ok := item[k]; ok && v != nil && v != "" {
			out[k] = v
		}
	}
	if price, ok := item["price"].(map[string]any); ok {
		if vs, ok := price["viewSection"].(map[string]any); ok {
			for _, k := range []string{"priceString", "fullPriceString"} {
				if v, ok := vs[k].(string); ok && v != "" {
					out[k] = v
				}
			}
		}
	}
	if avail, ok := item["availability"].(map[string]any); ok {
		if v, ok := avail["available"].(bool); ok {
			out["available"] = v
		}
		if v, ok := avail["stockLevel"].(string); ok && v != "" {
			out["stockLevel"] = v
		}
	}
	return out
}
