package mcp

import (
	"context"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/cli"
)

// RegisterNovelTools adds purpose-built MCP tools for the novel Costco receipt
// features. These bypass the command mirror and call the API directly with
// typed schemas, giving agents structured parameters and results.
func RegisterNovelTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("receipts",
			mcplib.WithDescription("List Costco in-warehouse, gas, and carwash receipts for a date range. Returns summary rows (date, warehouse, totals, barcode). Use receipt_detail for full line items."),
			mcplib.WithString("since", mcplib.Description("Start date (YYYY-MM-DD) or duration (30d, 6mo, 1y). Default: 2 years ago.")),
			mcplib.WithString("until", mcplib.Description("End date (YYYY-MM-DD). Default: today.")),
			mcplib.WithNumber("years", mcplib.Description("Lookback in years when since is not set (default 2).")),
			mcplib.WithString("type", mcplib.Description("Filter by channel: all, warehouse, gas, carwash (default all).")),
			mcplib.WithNumber("limit", mcplib.Description("Max receipts to return (0 = all).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleReceipts,
	)

	s.AddTool(
		mcplib.NewTool("history_depth",
			mcplib.WithDescription("Discover how far back your Costco receipts actually go, past the website's 2-year UI cap. Steps startDate backward in widening windows and reports where your earliest receipt stops moving."),
			mcplib.WithNumber("max_years", mcplib.Description("Maximum years back to probe (default 16).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleHistoryDepth,
	)

	s.AddTool(
		mcplib.NewTool("spend",
			mcplib.WithDescription("Roll up Costco spend by month, warehouse, or department over a date range. Returns aggregated rows with count and total per bucket."),
			mcplib.WithString("by", mcplib.Required(), mcplib.Description("Group spend by: month, warehouse, or department.")),
			mcplib.WithString("since", mcplib.Description("Start date (YYYY-MM-DD) or duration (30d, 6mo, 1y).")),
			mcplib.WithString("until", mcplib.Description("End date (YYYY-MM-DD). Default: today.")),
			mcplib.WithNumber("years", mcplib.Description("Lookback in years when since is not set (default 2).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleSpend,
	)

	s.AddTool(
		mcplib.NewTool("item_history",
			mcplib.WithDescription("Track one item's unit price across every receipt over time. Matches by description, item number, or UPC. Rows sorted oldest-first to show price drift."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Item description, item number, or UPC to search for (case-insensitive).")),
			mcplib.WithString("since", mcplib.Description("Start date (YYYY-MM-DD) or duration.")),
			mcplib.WithString("until", mcplib.Description("End date (YYYY-MM-DD). Default: today.")),
			mcplib.WithNumber("years", mcplib.Description("Lookback in years (default 3).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleItemHistory,
	)

	s.AddTool(
		mcplib.NewTool("savings",
			mcplib.WithDescription("Total the instant savings and coupon discounts captured across receipts over a date range."),
			mcplib.WithString("since", mcplib.Description("Start date (YYYY-MM-DD) or duration.")),
			mcplib.WithString("until", mcplib.Description("End date (YYYY-MM-DD). Default: today.")),
			mcplib.WithNumber("years", mcplib.Description("Lookback in years (default 2).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleSavings,
	)

	s.AddTool(
		mcplib.NewTool("returns_window",
			mcplib.WithDescription("Flag recently purchased items still inside a return window you set. Lists items with days remaining, sorted by urgency (fewest days left first)."),
			mcplib.WithNumber("days", mcplib.Description("Return-window length in days (default 90).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleReturnsWindow,
	)
}

func argStr(args map[string]any, key, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

func argInt(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func handleReceipts(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	since := argStr(args, "since", "")
	until := argStr(args, "until", "")
	years := argInt(args, "years", 2)
	typ := argStr(args, "type", "all")
	limit := argInt(args, "limit", 0)

	start, end, err := cli.ResolveRange(since, until, years)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	receipts, err := cli.FetchReceiptsWithClient(ctx, c, start, end)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	rows := cli.SummarizeReceipts(receipts, typ)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return toolResultJSON(map[string]any{
		"range":    map[string]string{"start": start, "end": end},
		"count":    len(rows),
		"receipts": rows,
	})
}

func handleHistoryDepth(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	maxYears := argInt(args, "max_years", 16)
	if maxYears <= 0 {
		return mcplib.NewToolResultError("max_years must be positive"), nil
	}
	result, err := cli.ProbeHistoryDepth(ctx, c, maxYears)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return toolResultJSON(result)
}

func handleSpend(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	by := argStr(args, "by", "month")
	since := argStr(args, "since", "")
	until := argStr(args, "until", "")
	years := argInt(args, "years", 2)

	start, end, err := cli.ResolveRange(since, until, years)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	receipts, err := cli.FetchReceiptsWithClient(ctx, c, start, end)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	rows, err := cli.AggregateSpendFromReceipts(receipts, by)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return toolResultJSON(map[string]any{
		"range": map[string]string{"start": start, "end": end},
		"by":    by,
		"rows":  rows,
	})
}

func handleItemHistory(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	query := argStr(args, "query", "")
	if query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}
	since := argStr(args, "since", "")
	until := argStr(args, "until", "")
	years := argInt(args, "years", 3)

	start, end, err := cli.ResolveRange(since, until, years)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	receipts, err := cli.FetchReceiptsWithClient(ctx, c, start, end)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	rows := cli.MatchItemHistoryFromReceipts(receipts, query)
	return toolResultJSON(map[string]any{
		"range":   map[string]string{"start": start, "end": end},
		"query":   query,
		"count":   len(rows),
		"history": rows,
	})
}

func handleSavings(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	since := argStr(args, "since", "")
	until := argStr(args, "until", "")
	years := argInt(args, "years", 2)

	start, end, err := cli.ResolveRange(since, until, years)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	receipts, err := cli.FetchReceiptsWithClient(ctx, c, start, end)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	view := cli.ComputeSavingsFromReceipts(receipts, start, end)
	return toolResultJSON(view)
}

func handleReturnsWindow(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	days := argInt(args, "days", 90)
	if days <= 0 {
		return mcplib.NewToolResultError("days must be a positive number"), nil
	}
	start := time.Now().AddDate(0, 0, -(days + 3)).Format("2006-01-02")
	end := time.Now().Format("2006-01-02")
	receipts, err := cli.FetchReceiptsWithClient(ctx, c, start, end)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	rows := cli.ItemsInReturnWindow(receipts, days)
	return toolResultJSON(map[string]any{
		"window_days": days,
		"count":       len(rows),
		"items":       rows,
	})
}
