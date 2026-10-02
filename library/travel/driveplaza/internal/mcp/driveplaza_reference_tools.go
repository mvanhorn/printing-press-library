package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/mcp/cobratree"
)

// ConfigureDrivePlazaTools replaces the generic HTML/raw-response handlers
// with the companion CLI's structured extraction, without editing templates.
func ConfigureDrivePlazaTools(s *server.MCPServer) {
	for _, ref := range []struct{ name, leaf, description string }{
		{"reference_rest_form", "rest-form", "Inspect official SA/PA form metadata and canonical links. Returns parsed canonical_url, title and links; use roads, sapa_facilities or sapa_list for planning facts."},
		{"reference_route_form", "route-form", "Inspect official route form metadata and canonical links. Returns parsed canonical_url, title and links; use route for source quote alternatives."},
		{"reference_schedule", "schedule", "Inspect official planned-roadwork page links through the CLI's source filter. Use handoff for labeled operator, construction and ETC-lane destinations."},
	} {
		s.AddTool(mcplib.NewTool(ref.name, mcplib.WithDescription(ref.description), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false), mcplib.WithOpenWorldHintAnnotation(true)), drivePlazaReferenceHandler(ref.leaf))
	}
	s.AddTool(mcplib.NewTool("context", mcplib.WithDescription("Get Drive Plaza planning assumptions, source coverage, pagination and command routing. Call before selecting a planning tool."), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false)), drivePlazaContextHandler(s))
	s.AddTool(mcplib.NewTool("sql", mcplib.WithDescription("Inspect existing local framework state with read-only SELECT queries. Live Drive Plaza planning data is not mirrored into this store; use planning tools for current source facts."), mcplib.WithString("query", mcplib.Required(), mcplib.Description("Read-only SELECT or WITH...SELECT query against an existing local store.")), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false)), handleSQL)
}

func drivePlazaReferenceHandler(leaf string) func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if len(req.GetArguments()) > 0 {
			return mcplib.NewToolResultError("This reference tool accepts no arguments."), nil
		}
		path, err := cobratree.SiblingCLIPath()
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("locate companion CLI: %v", err)), nil
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		args := []string{"reference", leaf, "--agent", "--no-cache", "--no-learn"}
		if leaf != "schedule" {
			args = append(args, "--select", "canonical_url,title,links")
		}
		result, err := cobratree.RunCLICommand(ctx, path, args)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		return cobratree.ToolResultFromCLICommand(result), nil
	}
}

func drivePlazaContextHandler(s *server.MCPServer) func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		result, err := handleContextResult(s, ctx, req)
		if err != nil {
			return nil, err
		}
		if len(result.Content) == 0 {
			return result, nil
		}
		text, ok := result.Content[0].(mcplib.TextContent)
		if !ok {
			return result, nil
		}
		var out map[string]any
		if err = json.Unmarshal([]byte(text.Text), &out); err != nil {
			return nil, err
		}
		out["archetype"] = "stateless-public-html-xml"
		out["tool_surface"] = "Planning commands use the companion CLI's bounded domain parsers. Reference tools return structured page metadata or filtered links. Local learning is separate from current source facts."
		out["query_tips"] = []string{
			"List pagination is local: limit defaults to 10, maximum 30; offset skips matching records in the current source response.",
			"Use interchanges to resolve source IC names, then route with an explicit JST schedule, source vehicle class and conditional ETC assumptions.",
			"Use sapa_list for road-specific directional summaries, then sapa_detail for source facility text and weekday hours.",
			"SA/PA filtering scans at most 500 records by default; max-scan-records and scanned_items describe coverage independently of limit.",
			"Live planning commands have no offline mirror. Missing facts are null and partial Japanese enrichment appears in meta.warnings.",
			"Notices are dated advisory publications, not a comprehensive active-closure feed. Use handoff for official map and restriction pages.",
		}
		out["resources"] = []map[string]any{{"name": "reference", "description": "Parsed source page metadata and filtered handoffs; use domain tools for planning facts.", "endpoints": []string{"rest_form", "route_form", "schedule"}}}
		if features, ok := out["command_mirror_capabilities"].([]any); ok {
			for _, raw := range features {
				if feature, ok := raw.(map[string]any); ok && feature["name"] == "Directional facility evidence" {
					feature["command"] = "sapa_list"
					feature["cli_command"] = "sapa list"
					feature["mcp_tool"] = "sapa_list"
				}
			}
		}
		return toolResultJSON(out)
	}
}
