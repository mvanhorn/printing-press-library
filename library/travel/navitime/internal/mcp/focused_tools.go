package mcp

import (
	"context"
	"encoding/json"
	"sort"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterFocusedTools exposes the public NAVITIME workflows and context.
// Generated registration remains intact for regeneration and conformance tests.
func RegisterFocusedTools(s *server.MCPServer) {
	generatedRegisterTools(s)
	s.DeleteTools("search", "sql")
	s.AddTool(mcplib.NewTool("context",
		mcplib.WithDescription("Explain the focused NAVITIME workflows, source limits and available tools."),
		mcplib.WithReadOnlyHintAnnotation(true),
		mcplib.WithDestructiveHintAnnotation(false),
	), focusedContext(s))
}

func generatedRegisterTools(s *server.MCPServer) {
	RegisterTools(s)
}

func focusedContext(s *server.MCPServer) server.ToolHandlerFunc {
	return func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		type tool struct {
			Name       string  `json:"name"`
			CLICommand *string `json:"cli_command"`
		}
		tools := make([]tool, 0, len(s.ListTools()))
		for name, registered := range s.ListTools() {
			item := tool{Name: name}
			if registered.Tool.Meta != nil {
				if command, ok := registered.Tool.Meta.AdditionalFields["pp:cli-command"].(string); ok {
					item.CLICommand = &command
				}
			}
			tools = append(tools, item)
		}
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
		payload := struct {
			Source              string            `json:"source"`
			CapabilitiesCommand string            `json:"capabilities_command"`
			ToolCount           int               `json:"tool_count"`
			Tools               []tool            `json:"tools"`
			Guidance            map[string]string `json:"guidance"`
		}{
			Source:              "NAVITIME Japan Travel public website only",
			CapabilitiesCommand: "navitime-pp-cli capabilities",
			ToolCount:           len(tools), Tools: tools,
			Guidance: map[string]string{
				"verification": "Run capabilities for dated access and verification status.",
				"places":       "Resolve bilingual candidates before choosing an explicit station or spot reference; names can be ambiguous.",
				"time":         "Choose exactly one dated depart-at, arrive-by, first-on or last-on mode. Offset-free times mean Asia/Tokyo.",
				"availability": "Timetable information does not establish live operational status or seat availability.",
				"fares":        "Published source fares retain base and optional seat supplements; they do not establish pass-holder out-of-pocket cost.",
				"pagination":   "Source pagination is not verified. passes list applies limit and offset locally to the advertised catalogue.",
				"snapshots":    "routes show reads a historical stored snapshot without another HTTP request.",
				"refresh":      "refresh and data-source live bypass response cache reads; successful public responses may update the cache.",
				"no_cache":     "no-cache skips cache reads and writes, including snapshot persistence; it does not change source availability.",
			},
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return mcplib.NewToolResultText(string(data)), nil
	}
}
