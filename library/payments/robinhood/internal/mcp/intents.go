// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterIntents adds small read-only workflow tools above the raw endpoint
// mirror. These keep common agent flows to one MCP call while the endpoint
// mirror remains available for exact API work.
func RegisterIntents(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("robinhood_crypto_account_snapshot",
			mcplib.WithDescription("Fetch a read-only crypto account snapshot: accounts, holdings, and recent orders. This does not place or cancel orders."),
			mcplib.WithString("account_number", mcplib.Description("Optional v2 crypto account number. When provided, v2 holdings and orders are queried for that account.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleCryptoAccountSnapshot,
	)

	s.AddTool(
		mcplib.NewTool("robinhood_crypto_market_snapshot",
			mcplib.WithDescription("Fetch a read-only crypto market snapshot: trading pairs plus best bid/ask, optionally narrowed to one symbol such as BTC-USD."),
			mcplib.WithString("symbol", mcplib.Description("Optional trading pair symbol, for example BTC-USD.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleCryptoMarketSnapshot,
	)
}

func handleCryptoAccountSnapshot(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	accountNumber, _ := args["account_number"].(string)
	out := map[string]any{}

	if err := getJSON(ctx, c, "/api/v2/crypto/trading/accounts/", nil, out, "accounts"); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("accounts failed: %v", err)), nil
	}

	holdingsParams := map[string]string{}
	ordersParams := map[string]string{}
	if strings.TrimSpace(accountNumber) != "" {
		holdingsParams["account_number"] = strings.TrimSpace(accountNumber)
		ordersParams["account_number"] = strings.TrimSpace(accountNumber)
		if err := getJSON(ctx, c, "/api/v2/crypto/trading/holdings/", holdingsParams, out, "holdings"); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("holdings failed: %v", err)), nil
		}
		if err := getJSON(ctx, c, "/api/v2/crypto/trading/orders/", ordersParams, out, "orders"); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("orders failed: %v", err)), nil
		}
	} else {
		if err := getJSON(ctx, c, "/api/v1/crypto/trading/holdings/", nil, out, "holdings"); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("holdings failed: %v", err)), nil
		}
		if err := getJSON(ctx, c, "/api/v1/crypto/trading/orders/", nil, out, "orders"); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("orders failed: %v", err)), nil
		}
	}

	return jsonToolResult(out)
}

func handleCryptoMarketSnapshot(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	symbol, _ := args["symbol"].(string)
	params := map[string]string{}
	if strings.TrimSpace(symbol) != "" {
		params["symbol"] = strings.ToUpper(strings.TrimSpace(symbol))
	}
	out := map[string]any{}
	if err := getJSON(ctx, c, "/api/v2/crypto/trading/trading_pairs/", params, out, "trading_pairs"); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("trading_pairs failed: %v", err)), nil
	}
	if err := getJSON(ctx, c, "/api/v2/crypto/marketdata/best_bid_ask/", params, out, "best_bid_ask"); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("best_bid_ask failed: %v", err)), nil
	}
	return jsonToolResult(out)
}

type mcpGetter interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

func getJSON(ctx context.Context, c mcpGetter, path string, params map[string]string, out map[string]any, key string) error {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		out[key+"_raw"] = string(raw)
		return nil
	}
	out[key] = decoded
	return nil
}

func jsonToolResult(value any) (*mcplib.CallToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("encode result: %v", err)), nil
	}
	return mcplib.NewToolResultText(string(data)), nil
}
