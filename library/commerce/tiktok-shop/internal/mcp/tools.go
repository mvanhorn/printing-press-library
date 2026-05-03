// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// MCP exposes the same safe v1 read surface as the CLI; mutations stay disabled.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
)

func RegisterTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("context", mcplib.WithDescription("Get TikTok Shop Printing Press v1 context, auth requirements, confirmed docs, and deferred risks.")),
		handleContext,
	)
	s.AddTool(
		mcplib.NewTool("shops_info", mcplib.WithDescription("List shops authorized for this app and token. Confirmed by official Get Authorized Shops 202309 docs.")),
		handleAuthorizedShops,
	)
	s.AddTool(
		mcplib.NewTool("orders_list",
			mcplib.WithDescription("Search orders. Returns raw TikTok Shop JSON; buyer/order data is PII."),
			mcplib.WithNumber("limit", mcplib.Description("Orders per page, official range 1-100")),
			mcplib.WithString("page_token", mcplib.Description("Opaque next page token")),
			mcplib.WithString("status", mcplib.Description("Optional order status")),
		),
		makeSearchHandler("POST", "/order/202309/orders/search", 20, 100, client.OrderListDocsURL),
	)
	s.AddTool(
		mcplib.NewTool("orders_get",
			mcplib.WithDescription("Get one order by ID. Returns raw TikTok Shop JSON; order data is PII."),
			mcplib.WithString("order_id", mcplib.Required(), mcplib.Description("TikTok Shop order ID")),
		),
		handleOrderGet,
	)
	s.AddTool(
		mcplib.NewTool("products_list",
			mcplib.WithDescription("Search products/listings. Confirmed by official Search Products 202309 docs."),
			mcplib.WithNumber("limit", mcplib.Description("Products per page, official range 1-100")),
			mcplib.WithString("page_token", mcplib.Description("Opaque next page token")),
			mcplib.WithString("status", mcplib.Description("Optional product status")),
		),
		makeSearchHandler("POST", "/product/202309/products/search", 50, 100, client.ProductSearchDocsURL),
	)
	s.AddTool(
		mcplib.NewTool("products_get",
			mcplib.WithDescription("Get one product by ID. Confirmed by official Get Product 202309 docs."),
			mcplib.WithString("product_id", mcplib.Required(), mcplib.Description("TikTok Shop product ID")),
		),
		handleProductGet,
	)
	s.AddTool(
		mcplib.NewTool("inventory_get",
			mcplib.WithDescription("Get inventory for one SKU ID using official Inventory Search 202309 docs."),
			mcplib.WithString("sku_id", mcplib.Required(), mcplib.Description("TikTok Shop SKU ID")),
		),
		handleInventoryGet,
	)
	s.AddTool(
		mcplib.NewTool("fulfillment_list",
			mcplib.WithDescription("Search packages. Confirmed by official Search Package 202309 docs."),
			mcplib.WithNumber("limit", mcplib.Description("Packages per page, official range 1-50")),
			mcplib.WithString("page_token", mcplib.Description("Opaque next page token")),
			mcplib.WithString("status", mcplib.Description("Optional package status")),
		),
		makeSearchHandler("POST", "/fulfillment/202309/packages/search", 20, 50, client.PackageSearchDocsURL),
	)
	s.AddTool(
		mcplib.NewTool("fulfillment_get",
			mcplib.WithDescription("Get one package by ID. Confirmed by official Get Package Detail 202309 docs."),
			mcplib.WithString("package_id", mcplib.Required(), mcplib.Description("TikTok Shop package ID")),
		),
		handlePackageGet,
	)
	s.AddTool(
		mcplib.NewTool("fulfillment_warehouses", mcplib.WithDescription("List seller warehouses. Confirmed by official Get Warehouse List 202309 docs.")),
		makeReadHandler("GET", "/logistics/202309/warehouses", nil),
	)
	s.AddTool(
		mcplib.NewTool("inventory_update_status", mcplib.WithDescription("Explain why inventory update is deferred in v1 despite confirmed endpoint docs.")),
		handleInventoryUpdateStatus,
	)
}

func handleContext(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	out := map[string]any{
		"name":    "tiktok-shop-pp-mcp",
		"safe_v1": true,
		"auth": map[string]any{
			"required_env":    []string{config.EnvAppKey, config.EnvAppSecret, config.EnvAccessToken},
			"shop_scoped_env": []string{config.EnvShopCipher},
			"token_header":    client.AccessTokenHeader,
			"docs":            []string{client.AuthorizationOverviewURL, client.SigningDocsURL},
		},
		"confirmed_read_tools": []string{"shops_info", "orders_list", "orders_get", "products_list", "products_get", "inventory_get", "fulfillment_list", "fulfillment_get", "fulfillment_warehouses"},
		"deferred":             []string{"inventory update", "returns/refunds", "shipping label mutations", "product create/update/delete", "finance/settlements", "webhook registration"},
	}
	data, _ := json.Marshal(out)
	return mcplib.NewToolResultText(string(data)), nil
}

func handleAuthorizedShops(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	raw, err := c.AuthorizedShops(ctx)
	return result(raw, err)
}

func handleOrderGet(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := requiredString(req.GetArguments(), "order_id")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	q := url.Values{}
	q.Set("ids", id)
	return callOpenAPI(ctx, "GET", "/order/202309/orders", q, nil)
}

func handleProductGet(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := requiredString(req.GetArguments(), "product_id")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return callOpenAPI(ctx, "GET", "/product/202309/products/"+url.PathEscape(id), nil, nil)
}

func handleInventoryGet(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := requiredString(req.GetArguments(), "sku_id")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return callOpenAPI(ctx, "POST", "/product/202309/inventory/search", nil, map[string]any{"sku_ids": []string{id}})
}

func handlePackageGet(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	id, err := requiredString(req.GetArguments(), "package_id")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return callOpenAPI(ctx, "GET", "/fulfillment/202309/packages/"+url.PathEscape(id), nil, nil)
}

func handleInventoryUpdateStatus(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	out := map[string]any{
		"status":  "deferred_mutation",
		"doc_url": client.InventoryUpdateDocsURL,
		"reason":  "endpoint is confirmed, but v1 has no official idempotency guarantee; retrying inventory mutations could corrupt stock levels",
	}
	data, _ := json.Marshal(out)
	return mcplib.NewToolResultText(string(data)), nil
}

func makeSearchHandler(method, path string, defaultLimit, maxLimit int, docURL string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := req.GetArguments()
		limit := numberArg(args, "limit", defaultLimit)
		if limit < 1 || limit > maxLimit {
			return mcplib.NewToolResultError(fmt.Sprintf("limit must be between 1 and %d; see %s", maxLimit, docURL)), nil
		}
		q := url.Values{}
		q.Set("page_size", strconv.Itoa(limit))
		setQueryArg(q, args, "page_token")
		body := map[string]any{}
		if status, ok := args["status"].(string); ok && status != "" {
			if strings.Contains(path, "/order/") {
				body["order_status"] = status
			} else if strings.Contains(path, "/fulfillment/") {
				body["package_status"] = status
			} else {
				body["status"] = status
			}
		}
		return callOpenAPI(ctx, method, path, q, body)
	}
}

func makeReadHandler(method, path string, query url.Values) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		return callOpenAPI(ctx, method, path, query, nil)
	}
}

func callOpenAPI(ctx context.Context, method, path string, query url.Values, body any) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	raw, err := c.DoOpenAPI(ctx, method, path, query, body)
	return result(raw, err)
}

func result(raw json.RawMessage, err error) (*mcplib.CallToolResult, error) {
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(raw)), nil
}

func newMCPClient() (*client.Client, error) {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "tiktok-shop-pp-cli", "config.toml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return client.New(cfg, 30*time.Second), nil
}

func requiredString(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func numberArg(args map[string]any, key string, fallback int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func setQueryArg(values url.Values, args map[string]any, key string) {
	if value, ok := args[key].(string); ok && value != "" {
		values.Set(key, value)
	}
}
