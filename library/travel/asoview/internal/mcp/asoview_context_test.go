package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"strings"
	"testing"
)

func TestAsoviewContextUsesActualDiscoveryContract(t *testing.T) {
	ctx := asoviewContext(map[string]any{"resources": []map[string]any{{"name": "source", "syncable": true, "searchable": true}}})
	row := ctx["resources"].([]map[string]any)[0]
	if row["syncable"] != false || row["searchable"] != false {
		t.Fatal(row)
	}
	tips := strings.Join(ctx["query_tips"].([]string), " ")
	if !strings.Contains(tips, "--cursor") || !strings.Contains(tips, "ten matches") || strings.Contains(tips, "Run sync") {
		t.Fatal(tips)
	}
}

func TestAsoviewContextHookSurvivesReprint(t *testing.T) {
	s := server.NewMCPServer("asoview-review-test", "0.1")
	result, err := handleContextResult(s, context.Background(), mcplib.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal([]byte(mcpTextContent(t, result)), &got); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got["query_tips"])
	if strings.Contains(string(raw), "Run sync") || !strings.Contains(string(raw), "--cursor") {
		t.Fatal(string(raw))
	}
}
