package mcp

import "github.com/mark3labs/mcp-go/server"

var michiOptionalStateCommands = map[string]bool{
	"learnings candidates": true, "learnings list": true, "learnings stats": true,
	"playbook list": true, "recall": true, "workflow status": true,
}

// These explicit optional helpers can create/migrate local stores or append
// telemetry/journals. Declare that fact instead of changing their recall or
// teaching behavior with a blanket no-learn switch. Native mirrors remain
// read-only; direct SQL/context do not pass through this override.
func registerMichiOptionalStateAnnotations(s *server.MCPServer) {
	for _, registered := range s.ListTools() {
		if registered.Tool.Meta == nil {
			continue
		}
		command, _ := registered.Tool.Meta.AdditionalFields["pp:cli-command"].(string)
		if !michiOptionalStateCommands[command] {
			continue
		}
		tool := registered.Tool
		no := false
		tool.Annotations.ReadOnlyHint = &no
		tool.Annotations.DestructiveHint = &no
		tool.Annotations.OpenWorldHint = &no
		tool.Description += " This optional helper can write local learning or state records."
		s.AddTool(tool, registered.Handler)
	}
}
