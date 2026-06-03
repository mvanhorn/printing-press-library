// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
)

// classifyMCPAPIError maps API errors to MCP tool results with actionable
// hints. Switches on the typed `*client.APIError.StatusCode` (not on
// `err.Error()` substring matching) so any reformatting of `APIError.Error()`
// upstream doesn't silently strip the classification.
//
// For DELETE methods, 404 collapses to a "no-op" text result so agents
// treating delete as idempotent see the expected shape.
func classifyMCPAPIError(err error, method string) *mcplib.CallToolResult {
	msg := err.Error()
	var apiE *client.APIError
	status := 0
	if errors.As(err, &apiE) {
		status = apiE.StatusCode
	}
	switch status {
	case 409:
		return mcplib.NewToolResultText("already exists (no-op)")
	case 400:
		if cliutil.LooksLikeAuthError(msg) {
			return mcplib.NewToolResultError("authentication error: " + cliutil.SanitizeErrorBody(msg) +
				"\nhint: the API rejected the request — this usually means auth is missing or invalid." +
				"\n      Set credentials: export GORGIAS_USERNAME=<email> GORGIAS_API_KEY=<key>" +
				"\n      Get a key at: https://docs.gorgias.com/en-US/rest-api-208286" +
				"\n      Settings → REST API → API key (use your email as username)" +
				"\n      Run 'gorgias-pp-cli doctor' to check auth status.")
		}
		return mcplib.NewToolResultError(msg)
	case 401:
		return mcplib.NewToolResultError("authentication failed: " + cliutil.SanitizeErrorBody(msg) +
			"\nhint: check your API key." +
			"\n      Set credentials: export GORGIAS_USERNAME=<email> GORGIAS_API_KEY=<key>" +
			"\n      Get a key at: https://docs.gorgias.com/en-US/rest-api-208286" +
			"\n      Settings → REST API → API key (use your email as username)" +
			"\n      Run 'gorgias-pp-cli doctor' to check auth status.")
	case 403:
		return mcplib.NewToolResultError("permission denied: " + cliutil.SanitizeErrorBody(msg) +
			"\nhint: your credentials are valid but lack access to this resource." +
			"\n      Set credentials: export GORGIAS_USERNAME=<email> GORGIAS_API_KEY=<key>" +
			"\n      Get a key at: https://docs.gorgias.com/en-US/rest-api-208286" +
			"\n      Settings → REST API → API key (use your email as username)" +
			"\n      Run 'gorgias-pp-cli doctor' to check auth status.")
	case 404:
		if method == "DELETE" {
			return mcplib.NewToolResultText("already deleted (no-op)")
		}
		return mcplib.NewToolResultError("not found: " + msg)
	case 429:
		return mcplib.NewToolResultError("rate limited: " + msg)
	default:
		return mcplib.NewToolResultError(msg)
	}
}

// registeredToolCount records the total number of tools registered into the
// MCP server in the most recent call to RegisterTools. handleContext reads
// this so introspecting agents see a number that matches the live surface,
// not a hand-edited constant that decays whenever a workflow command is
// added or hidden.
var registeredToolCount int

// RegisterTools registers all API operations as MCP tools.
func RegisterTools(s *server.MCPServer) {
	// Code-orchestration mode — the full surface is covered by two tools
	// (<api>_search + <api>_execute). Endpoint-mirror tools are suppressed.
	orchCount := RegisterCodeOrchestrationTools(s)
	// Search tool — faster than iterating list endpoints for finding specific items
	s.AddTool(
		mcplib.NewTool("search",
			mcplib.WithDescription("Full-text search the local SQLite mirror (FTS5). Use for ticket/message text search — Gorgias's POST /search indexes customers/agents/tags only, NOT tickets, so even --data-source live falls back to local for those scopes. Always run `sync` first to populate the index; subsequent searches run subsecond. Pairs with `sql` for ad-hoc joins."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("Search query (supports FTS5 syntax: AND, OR, NOT, quotes for phrases)")),
			mcplib.WithNumber("limit", mcplib.Description("Max results (default 25)")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleSearch,
	)
	// SQL tool — ad-hoc analysis on synced data without API calls
	s.AddTool(
		mcplib.NewTool("sql",
			mcplib.WithDescription("Run read-only SQL against the local SQLite mirror. Single generic table: `resources` (columns: resource_type, id, data, synced_at). Filter by `resource_type` ('tickets', 'customers', 'tags', etc.) to scope a query. Response bodies are JSON-encoded in the `data` TEXT column; access fields via `json_extract(data, '$.field')`. Always paginate with LIMIT — the mirror can carry tens of thousands of rows. Read-only: write queries are rejected. Requires `sync` first."),
			mcplib.WithString("query", mcplib.Required(), mcplib.Description("SQL query (SELECT or WITH...SELECT). Tables match resource names.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleSQL,
	)

	// Context tool — front-loaded domain knowledge for agents.
	// Call this first to understand the API taxonomy, query patterns, and capabilities.
	s.AddTool(
		mcplib.NewTool("context",
			mcplib.WithDescription("Return the full machine-readable descriptor of this CLI: auth config, env vars, novel features, resource list, MCP transport, tool inventory. Call once at the start of a long session to load the descriptor into your working memory. Avoid re-calling within the same session — the descriptor doesn't change at runtime. Use over re-fetching individual tool descriptions because `context` summarizes WHEN to reach for each surface (live API vs local mirror vs compound workflow vs MCP gateway)."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
		),
		handleContext,
	)

	// Runtime Cobra-tree mirror — exposes every user-facing command that is
	// not already covered by a typed endpoint or framework MCP tool.
	cobraCount := cobratree.RegisterAll(s, cli.RootCmd(), cobratree.SiblingCLIPath)

	// 3 framework tools registered directly above (search, sql, context) +
	// the orchestration gateway tools + the cobra-tree mirror tools.
	registeredToolCount = 3 + orchCount + cobraCount
}

func newMCPClient() (*client.Client, error) {
	// config.Load("") resolves through XDG_CONFIG_HOME, so the MCP server and
	// the CLI agree on which config file to read even when the user has
	// XDG_CONFIG_HOME pointed somewhere non-default.
	cfg, err := config.Load("")
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	c := client.New(cfg, 30*time.Second, 0)
	// Agents calling through MCP need fresh data every call. The on-disk
	// response cache survives across MCP server invocations, so a
	// DELETE/PATCH followed by a GET would otherwise return the
	// pre-mutation snapshot for up to the cache TTL. The interactive CLI
	// constructs its own client and is unaffected.
	c.NoCache = true
	return c, nil
}

// dbPath returns the XDG_DATA_HOME-relative SQLite path. Must match the
// CLI's defaultDBPath so both binaries read from the same file even when
// XDG_DATA_HOME is overridden.
func dbPath() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "gorgias-pp-cli", "data.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gorgias-pp-cli", "data.db")
}

func handleSearch(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}

	limit := 25
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	db, err := store.OpenReadOnly(dbPath())
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("opening database: %v", err)), nil
	}
	defer db.Close()

	results, err := db.Search(query, limit)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

// The read-only SQL gate is defined once in internal/cliutil/sqlgate.go and
// shared with the CLI `sql` command. The MCP tool advertises
// ReadOnlyHintAnnotation(true) to hosts; if a write slips through, hosts
// auto-approve it. See cliutil.ValidateReadOnlySQL for the contract.

func handleSQL(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}

	if err := cliutil.ValidateReadOnlySQL(query); err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	db, err := store.OpenReadOnly(dbPath())
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("opening database: %v", err)), nil
	}
	defer db.Close()

	// Use QueryContext via the same *sql.DB accessor the CLI `sql` command uses
	// so request cancellation/timeout propagates to SQLite — keeping the MCP and
	// CLI read paths identical rather than divergent.
	rows, err := db.DB().QueryContext(ctx, query)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if scanErr := rows.Scan(ptrs...); scanErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("scan row: %v", scanErr)), nil
		}
		row := make(map[string]any)
		for i, col := range cols {
			row[col] = values[i]
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("reading rows: %v", err)), nil
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

func handleContext(_ context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	ctx := map[string]any{
		"api":         "gorgias",
		"description": "Every Gorgias support workflow, agent-native, in one binary.",
		"archetype":   "customer-support",
		// tool_count reflects what the MCP server actually exposed at
		// registration time. The full ~108-endpoint surface is reachable
		// from the orchestration gateway, not as 108 separate tools.
		"tool_count":           registeredToolCount,
		"underlying_endpoints": 108,
		// tool_surface tells agents which surface a capability lives on.
		"tool_surface": "MCP server runs in code-orchestration mode: 2 orchestration tools (gorgias_search, gorgias_execute), 3 framework tools (search, sql, context), plus a runtime cobra-mirror of agent-relevant CLI verbs (workflow_*, sync, analytics, pm_*, export, import, tail). Reach the full Gorgias endpoint surface via gorgias_execute rather than expecting per-endpoint typed tools.",
		"auth": map[string]any{
			"type": "api_key",
			"env_vars": []map[string]any{
				{
					"name":      "GORGIAS_USERNAME",
					"kind":      "per_call",
					"required":  true,
					"sensitive": false,
				},
				{
					"name":        "GORGIAS_API_KEY",
					"kind":        "per_call",
					"required":    true,
					"sensitive":   true,
					"description": "Set to your API credential.",
				},
			},
			"key_url":      "https://docs.gorgias.com/en-US/rest-api-208286",
			"instructions": "Settings → REST API → API key (use your email as username)",
		},
		"resources": []map[string]any{
			{
				"name":        "account",
				"description": "Account-level settings and tenant metadata",
				"endpoints":   []string{"get", "settings-create", "settings-list", "settings-update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "custom-fields",
				"description": "Define and manage custom fields on tickets and customers",
				"endpoints":   []string{"create", "get", "list", "update", "update-all"},
				"searchable":  true,
			},
			{
				"name":        "customers",
				"description": "Read and write Gorgias customer records (CRM core)",
				"endpoints":   []string{"create", "custom-fields-list", "custom-fields-set", "custom-fields-set-all", "custom-fields-unset", "data-update", "delete", "delete-all", "get", "list", "merge", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "events",
				"description": "Audit log of who-changed-what across tickets, customers, settings",
				"endpoints":   []string{"get", "list"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "gorgias-jobs",
				"description": "Schedule and track async Gorgias jobs (bulk exports, macro applies)",
				"endpoints":   []string{"create", "delete", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "integrations",
				"description": "Install and configure third-party integrations (Shopify, SMS, social)",
				"endpoints":   []string{"create", "delete", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "macros",
				"description": "Reusable canned-reply templates with variables and actions",
				"endpoints":   []string{"archive", "create", "delete", "get", "list", "unarchive", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "messages",
				"description": "Read messages across tickets (account-wide listing)",
				"endpoints":   []string{"list"},
				"syncable":    true,
			},
			{
				"name":        "phone",
				"description": "Voice calls, call events, and recorded audio",
				"endpoints":   []string{"call-events-get", "call-events-list", "call-recordings-delete", "call-recordings-get", "call-recordings-list", "calls-get", "calls-list"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "pickups",
				"description": "Delete pickup logistics records",
				"endpoints":   []string{"delete"},
			},
			{
				"name":        "reporting",
				"description": "Reporting and analytics over support data",
				"endpoints":   []string{"stats"},
			},
			{
				"name":        "rules",
				"description": "Automation rules: route, tag, auto-reply, escalate on incoming tickets",
				"endpoints":   []string{"create", "delete", "get", "list", "set-priorities", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "satisfaction-surveys",
				"description": "CSAT survey definitions and customer ratings/comments",
				"endpoints":   []string{"create", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "tags",
				"description": "Ticket tags — the labels that drive routing rules and reporting",
				"endpoints":   []string{"create", "delete", "delete-all", "get", "list", "merge", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "teams",
				"description": "Agent teams: how tickets are grouped and routed for assignment",
				"endpoints":   []string{"create", "delete", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "ticket-search",
				"description": "Search across tickets, customers, messages, etc.",
				"endpoints":   []string{"query"},
				"searchable":  true,
			},
			{
				"name":        "tickets",
				"description": "Read and write Gorgias tickets, messages, and tag assignments",
				"endpoints":   []string{"create", "custom-fields-list", "custom-fields-set", "custom-fields-set-all", "custom-fields-unset", "delete", "get", "list", "messages-create", "messages-delete", "messages-get", "messages-list", "messages-update", "tags-add", "tags-list", "tags-remove", "tags-replace", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "users",
				"description": "Agents and admin users on the Gorgias account",
				"endpoints":   []string{"create", "delete", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "views",
				"description": "Saved Gorgias inbox views (named filters used by agents)",
				"endpoints":   []string{"create", "delete", "get", "items-list", "items-update", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
			{
				"name":        "widgets",
				"description": "Configure on-site chat/contact widget instances",
				"endpoints":   []string{"create", "delete", "get", "list", "update"},
				"syncable":    true,
				"searchable":  true,
			},
		},
		"query_tips": []string{
			"Pagination uses cursor-based paging. Pass cursor parameter for subsequent pages.",
			"Control page size with the limit parameter (default 100).",
			"Use the sql tool for ad-hoc analysis on synced data. Run sync first to populate the local database.",
			"Use the search tool for full-text search across all synced resources. Faster than iterating list endpoints.",
			"Prefer sql/search over repeated API calls when the data is already synced.",
			"Ticket sync since windows use documented order_by=updated_datetime:desc plus local filtering; do not add updated_datetime__gte unless Gorgias documents and live-accepts it.",
		},
		// Command-mirror capabilities are exposed through MCP by shelling out
		// to the companion CLI binary.
		"command_mirror_capabilities": []map[string]string{
			{"name": "Doctor with credential verification", "command": "gorgias-pp-cli doctor --json", "description": "Probes /account with the configured credentials and reports `credentials: valid` only when an authenticated call...", "rationale": "Every other Gorgias wrapper makes you set env vars and find out at first call whether they're right; doctor surfaces...", "via": "mcp-command-mirror"},
			{"name": "Local SQLite mirror for offline analytics", "command": "gorgias-pp-cli sync --resources tickets --since 7d && gorgias-pp-cli search 'refund' --agent", "description": "Syncs API data to a local SQLite DB so subsequent searches, analytics, and joins run without hitting the API. Ticket --since uses order_by=updated_datetime:desc plus local filtering.", "rationale": "Gorgias's API is rate-limited per account and pagination-heavy; mirroring locally turns a 30-second multi-page...", "via": "mcp-command-mirror"},
		},
		"playbook": []map[string]string{
			{"topic": "Doctor with credential verification", "insight": "Every other Gorgias wrapper makes you set env vars and find out at first call whether they're right; doctor surfaces auth failure as the first command you run."},
			{"topic": "Local SQLite mirror for offline analytics", "insight": "Gorgias's API is rate-limited per account and pagination-heavy; mirroring locally turns a 30-second multi-page search into a 50ms SQL query."},
			{"topic": "Finding stale work", "insight": "Use the stale command or sql query to find items not updated recently. More reliable than scanning list results manually."},
			{"topic": "Load analysis", "insight": "When analyzing team workload, filter by assignee and status. Raw counts without status filtering are misleading."},
			{"topic": "Bulk operations", "insight": "For bulk status changes, prefer update endpoints over delete+create. Most PM APIs track history on updates."},
		},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	return mcplib.NewToolResultText(string(data)), nil
}

// RegisterNovelFeatureTools is kept as a compatibility no-op for older MCP
// mains. New generated mains call RegisterTools only; RegisterTools now
// includes the runtime Cobra-tree mirror.
func RegisterNovelFeatureTools(s *server.MCPServer) {
	_ = s
}
