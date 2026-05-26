package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/internal/store"
)

func RegisterCodeOrchTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("adguard_home_search",
			mcplib.WithDescription("Full-text search across synced AdGuard Home data using FTS5. Returns matching resources from the local SQLite store."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query (supports FTS5 syntax: AND, OR, NOT, quotes for phrases)")),
			mcplib.WithString("resource_type", mcplib.Description("Optional resource type filter (e.g. clients, filtering, dhcp, rewrite, stats)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleCodeOrchSearch,
	)

	s.AddTool(
		mcplib.NewTool("adguard_home_execute",
			mcplib.WithDescription("Execute an AdGuard Home API call directly. Supports GET, POST, PUT, and DELETE methods."),
			mcplib.WithString("method", mcplib.Required(), mcplib.Description("HTTP method: GET, POST, PUT, or DELETE")),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("API path (e.g. /clients, /filtering/status, /dhcp/set_config)")),
			mcplib.WithString("body", mcplib.Description("Optional JSON body for POST/PUT/PATCH requests")),
			mcplib.WithDestructiveHintAnnotation(true),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		handleCodeOrchExecute,
	)
}

func handleCodeOrchSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}

	resourceType, _ := args["resource_type"].(string)

	db, err := store.OpenReadOnly(dbPath())
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("opening database: %v", err)), nil
	}
	defer db.Close()

	if resourceType != "" {
		return searchByType(db, query, resourceType)
	}

	results, err := db.Search(query, 25)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func searchByType(db *store.Store, query, resourceType string) (*mcplib.CallToolResult, error) {
	rows, err := db.Query(
		`SELECT r.data FROM resources r
		 JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
		 WHERE resources_fts MATCH ? AND r.resource_type = ?
		 ORDER BY rank
		 LIMIT 25`,
		query, resourceType,
	)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		results = append(results, json.RawMessage(data))
	}

	out, _ := json.MarshalIndent(results, "", "  ")
	return mcplib.NewToolResultText(string(out)), nil
}

func handleCodeOrchExecute(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	method, ok := args["method"].(string)
	if !ok || method == "" {
		return mcplib.NewToolResultError("method is required"), nil
	}
	method = strings.ToUpper(method)

	path, ok := args["path"].(string)
	if !ok || path == "" {
		return mcplib.NewToolResultError("path is required"), nil
	}

	var bodyArgs map[string]any
	if bodyStr, ok := args["body"].(string); ok && bodyStr != "" {
		if err := json.Unmarshal([]byte(bodyStr), &bodyArgs); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("invalid JSON body: %v", err)), nil
		}
	}

	c, err := newMCPClient()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	var data json.RawMessage
	switch method {
	case "GET":
		data, err = c.Get(path, nil)
	case "POST":
		data, _, err = c.PostWithParams(path, nil, bodyArgs)
	case "PUT":
		data, _, err = c.PutWithParams(path, nil, bodyArgs)
	case "DELETE":
		data, _, err = c.DeleteWithParams(path, nil)
	default:
		return mcplib.NewToolResultError("unsupported method: " + method), nil
	}

	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	if method == "GET" {
		trimmed := strings.TrimSpace(string(data))
		if len(trimmed) > 0 && trimmed[0] == '[' {
			var items []json.RawMessage
			if json.Unmarshal(data, &items) == nil {
				wrapped := map[string]any{
					"count": len(items),
					"items": items,
				}
				out, _ := json.Marshal(wrapped)
				return mcplib.NewToolResultText(string(out)), nil
			}
		}
	}

	return mcplib.NewToolResultText(string(data)), nil
}
