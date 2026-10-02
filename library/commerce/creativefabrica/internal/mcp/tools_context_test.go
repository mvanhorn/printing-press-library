package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestContextDescribesRuntimeToolAndPaginationSurface(t *testing.T) {
	result, err := handleContext(context.Background(), mcplib.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleContext: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(result.Content))
	}
	content, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("context content type = %T, want text", result.Content[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(content.Text), &payload); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if got := int(payload["tool_count"].(float64)); got != 14 {
		t.Fatalf("tool_count = %d, want 14", got)
	}
	text := strings.ToLower(fmt.Sprint(payload))
	for _, misleading := range []string{"cursor-based", "pass after", "synced catalog", "sqlite join", "local store"} {
		if strings.Contains(text, misleading) {
			t.Fatalf("context still advertises unavailable behavior %q", misleading)
		}
	}
	if !strings.Contains(text, "zero-based page") || !strings.Contains(text, "default 20") {
		t.Fatalf("context does not describe find pagination accurately: %s", content.Text)
	}
}
