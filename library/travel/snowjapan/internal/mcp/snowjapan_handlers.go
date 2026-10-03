// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"fmt"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/mcp/cobratree"
)

// The typed endpoint mirrors use the same factual adapters as the command
// tree. Returning generic raw HTML here would expose prose and omit chart data.
func snowjapanHTMLHandler(method, path string) server.ToolHandlerFunc {
	if method != "GET" {
		return nil
	}
	command := map[string][]string{
		"/":                                        []string{"reports", "list"},
		"/daily-snow-and-weather-reports/{id}":     []string{"reports", "get"},
		"/ski-areas-in-japan/{id}":                 []string{"resorts", "get"},
		"/insights/japan-ski-areas-statistics":     []string{"resorts", "list"},
		"/insights/{season}-ski-season-dates-sort": []string{"seasons", "list"},
	}[path]
	if command == nil {
		return nil
	}
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := append([]string{}, command...)
		args = append(args, "--agent", "--no-input")
		input := req.GetArguments()
		if path == "/daily-snow-and-weather-reports/{id}" || path == "/ski-areas-in-japan/{id}" {
			id, ok := input["id"].(string)
			if !ok || id == "" {
				return mcpToolError("id must be a nonempty exact source identity"), nil
			}
			// Treat the source identity strictly as an operand. A typed
			// input must never become a companion CLI's global flag.
			args = append(args, "--", id)
		}
		if path == "/insights/{season}-ski-season-dates-sort" {
			season, ok := input["season"].(string)
			if !ok || season == "" {
				return mcpToolError("season must be a completed YYYY-YYYY label"), nil
			}
			args = append(args, "--season", season)
		}
		binary, e := cobratree.SiblingCLIPath()
		if e != nil {
			return mcpToolError(fmt.Sprintf("companion factual CLI unavailable: %v", e)), nil
		}
		bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		out, e := cobratree.RunCLICommand(bounded, binary, args)
		if e != nil {
			return mcpToolError(e.Error()), nil
		}
		return cobratree.ToolResultFromCLICommand(out), nil
	}
}
