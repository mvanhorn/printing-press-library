package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestDrivePlazaContextRoutesCurrentCommands(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterTools(s)
	ConfigureDrivePlazaTools(s)
	result, e := drivePlazaContextHandler(s)(context.Background(), mcplib.CallToolRequest{})
	if e != nil {
		t.Fatal(e)
	}
	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatal("context is not structured text")
	}
	var out map[string]any
	if e = json.Unmarshal([]byte(text.Text), &out); e != nil {
		t.Fatal(e)
	}
	tips, _ := json.Marshal(out["query_tips"])
	if strings.Contains(string(tips), "cursor") || strings.Contains(string(tips), "sync first") || !strings.Contains(string(tips), "maximum 30") {
		t.Fatalf("wrong context tips: %s", tips)
	}
	features, _ := json.Marshal(out["command_mirror_capabilities"])
	if !strings.Contains(string(features), `"cli_command":"sapa list"`) || !strings.Contains(string(features), `"command":"sapa_list"`) || !strings.Contains(string(features), `"mcp_tool":"sapa_list"`) {
		t.Fatalf("wrong capability routing: %s", features)
	}
}
