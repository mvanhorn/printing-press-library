package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/mark3labs/mcp-go/server"
)

// Digg removed /api/trending/status in October 2026; the MCP server must
// not advertise a tool that can only fail.
func TestRegisterToolsOmitsTrendingStatus(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	if len(s.ListTools()) == 0 {
		t.Fatal("RegisterTools registered nothing")
	}
	if s.GetTool("trending_status") != nil {
		t.Fatal("trending_status is still registered")
	}
}

// Commands removed from the CLI because Digg stopped serving their data
// must not survive as MCP tools.
func TestRegisterToolsOmitsRemovedCommands(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	for _, name := range []string{"rankings_list", "top", "history"} {
		if s.GetTool(name) == nil {
			t.Errorf("%s should still be registered", name)
		}
	}
	for _, name := range []string{"events", "evidence", "sentiment", "crossref", "replaced", "rising", "rankings_emerging", "rankings_movers"} {
		if s.GetTool(name) != nil {
			t.Errorf("%s is still registered", name)
		}
	}
}

// The live story commands are the CLI's main surface; an agent connected
// to the MCP server must be able to call each of them.
func TestRegisterToolsExposesLiveStoryCommands(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	for _, name := range []string{"search", "posts", "story"} {
		if s.GetTool(name) == nil {
			t.Errorf("%s is not registered", name)
		}
	}
}

// `rankings` only groups `rankings list`; a tool of that name could do
// nothing but print help.
func TestRegisterToolsOmitsRankingsGroup(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	if s.GetTool("rankings") != nil {
		t.Error("the rankings group is registered as a tool")
	}
	if s.GetTool("rankings_list") == nil {
		t.Error("rankings_list should still be registered")
	}
}

// The context tool is what an agent reads first, so it must name the
// live story commands and count the tools the server really registers.
func TestContextNamesLiveStoryCommandsAndCountsTools(t *testing.T) {
	s := server.NewMCPServer("digg-test", "0")
	RegisterTools(s)
	res, err := handleContext(context.Background(), mcplib.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text, ok := res.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("context returned %T, want text", res.Content[0])
	}
	var got struct {
		ToolCount    int `json:"tool_count"`
		Capabilities []struct {
			Command string `json:"command"`
		} `json:"command_mirror_capabilities"`
	}
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, c := range got.Capabilities {
		listed[c.Command] = true
	}
	for _, name := range []string{"search", "posts", "story"} {
		if !listed[name] {
			t.Errorf("context does not list the %s command", name)
		}
	}
	if want := len(s.ListTools()); got.ToolCount != want {
		t.Errorf("context tool_count = %d, server registers %d tools", got.ToolCount, want)
	}
}
