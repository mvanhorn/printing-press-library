package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestSmartEXContextDoesNotRecommendAbsentToolsOrReferencePaging(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterTools(s)
	result, err := handleContext(s)(context.Background(), mcplib.CallToolRequest{})
	if err != nil || result.IsError {
		t.Fatalf("context failed: %+v %v", result, err)
	}
	var payload map[string]any
	if err = json.Unmarshal([]byte(mcpTextContent(t, result)), &payload); err != nil {
		t.Fatal(err)
	}
	tips := payload["query_tips"].([]any)
	for _, tip := range tips {
		text := tip.(string)
		for _, bad := range []string{"Run sync first", "Use the search tool", "Pass after parameter", "default 100"} {
			if strings.Contains(text, bad) {
				t.Fatalf("unsupported advice: %s", text)
			}
		}
	}
	if !strings.Contains(payload["local_storage"].(string), "no sync or search") {
		t.Fatal("stateless boundary missing")
	}
}
