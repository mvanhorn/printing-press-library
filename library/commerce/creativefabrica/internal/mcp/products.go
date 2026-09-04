// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cli"
)

func handleProductsSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	raw := ""
	if v, ok := args["requests"]; ok && v != nil {
		switch tv := v.(type) {
		case string:
			raw = tv
		default:
			b, err := json.Marshal(tv)
			if err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("parsing requests: %v", err)), nil
			}
			raw = string(b)
		}
	}
	if raw == "null" {
		raw = ""
	}
	reqs, err := cli.ParseAlgoliaRequests(raw)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	c := cli.NewCatalogClient(30*time.Second, defaultMCPRateLimit, "")
	results, err := c.Search(ctx, reqs...)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	data, err := json.Marshal(map[string]any{"results": results})
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcpToolResultText("POST", data), nil
}
