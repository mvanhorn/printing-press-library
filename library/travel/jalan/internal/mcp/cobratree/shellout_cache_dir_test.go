package cobratree

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cli"
)

func TestStayOffersMCPRejectsCallerCacheDirectory(t *testing.T) {
	root := cli.RootCmd()
	offers, _, err := root.Find([]string{"stay", "offers"})
	if err != nil || offers == nil {
		t.Fatalf("stay offers command missing: %v", err)
	}
	if offers.Flags().Lookup("cache-dir") == nil {
		t.Fatal("ordinary CLI cache-dir flag was removed")
	}

	registry := server.NewMCPServer("cache-dir-guard", "0.0.0")
	companion := filepath.Join(t.TempDir(), "must-not-execute")
	RegisterAll(registry, root, func() (string, error) { return companion, nil })
	toolName := ToolNameForCommand(registry, root, "stay offers")
	if toolName == "" {
		t.Fatal("stay offers MCP mirror missing")
	}
	registered := registry.GetTool(toolName)
	if registered == nil {
		t.Fatalf("registered %s tool missing", toolName)
	}
	properties := registered.Tool.InputSchema.Properties
	if _, present := properties["cache-dir"]; present {
		t.Fatal("MCP schema exposes a caller-chosen cache directory")
	}
	for _, allowed := range []string{"max-age", "refresh", "check-in"} {
		if _, ok := properties[allowed]; !ok {
			t.Fatalf("normal read-only flag %q disappeared from MCP schema", allowed)
		}
	}

	callerPath := filepath.Join(t.TempDir(), "caller-selected-cache")
	result, err := registered.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"cache-dir": callerPath}}})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(toolResultText(result), `unknown MCP parameter "cache-dir"`) {
		t.Fatalf("cache directory was not rejected before execution: %s", toolResultText(result))
	}
	if _, err := os.Stat(callerPath); !os.IsNotExist(err) {
		t.Fatalf("rejected cache directory was created: %v", err)
	}

	rawResult, err := registered.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"args": "--cache-dir=" + callerPath}}})
	if err != nil {
		t.Fatalf("raw-args handler returned transport error: %v", err)
	}
	if !rawResult.IsError || !strings.Contains(toolResultText(rawResult), "flag-like") || !strings.Contains(toolResultText(rawResult), "--cache-dir") {
		t.Fatalf("raw positional args smuggled cache-dir: %s", toolResultText(rawResult))
	}

	blocked := blockedStructuredArgsForCommand(offers)
	positionals := positionalArgsForCommand(offers, blocked)
	forwarded := cliArgsFromMCP(map[string]any{"cache-dir": callerPath, "max-age": "5m"}, cliFlagBlockedArgs(blocked, positionals))
	if !reflect.DeepEqual(forwarded, []string{"--max-age=5m"}) {
		t.Fatalf("cache-dir was forwarded to companion CLI: %v", forwarded)
	}
}
