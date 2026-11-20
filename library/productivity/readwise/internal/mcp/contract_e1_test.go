package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func e1Server(t *testing.T, handler http.HandlerFunc) *server.MCPServer {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("READWISE_API_TOKEN", "")
	t.Setenv("READWISE_MCP_READ_ONLY", "")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("READWISE_BASE_URL", srv.URL)
	cfg := filepath.Join(home, "fixture.toml")
	if err := os.WriteFile(cfg, []byte("api_token = \"e1-fixture\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("READWISE_CONFIG", cfg)
	s := server.NewMCPServer("Readwise", "test", server.WithToolCapabilities(false))
	RegisterTools(s)
	return s
}
func e1Call(t *testing.T, s *server.MCPServer, name string, args map[string]any) (*mcplib.CallToolResult, map[string]any, string) {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatal(name + " absent")
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].(mcplib.TextContent).Text
	var obj map[string]any
	_ = json.Unmarshal([]byte(text), &obj)
	return result, obj, text
}
func TestE1TypedPagesContiguous(t *testing.T) {
	docs := make([]map[string]any, 100)
	for i := range docs {
		docs[i] = map[string]any{"id": i, "content": strings.Repeat("x", 1100)}
	}
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"results": docs, "nextPageCursor": "upstream_next"})
	})
	t.Setenv("PP_MCP_MAX_BYTES", "32000")
	cursor := ""
	expected := 0
	for step := 0; step < 10 && expected < 100; step++ {
		result, obj, text := e1Call(t, s, "documents_list", map[string]any{"cursor": cursor})
		if result.IsError || len(text) > 32000 {
			t.Fatalf("bad page: %.200s", text)
		}
		rows, ok := obj["results"].([]any)
		if !ok || len(rows) == 0 {
			t.Fatalf("empty page: %.200s", text)
		}
		for _, row := range rows {
			id := int(row.(map[string]any)["id"].(float64))
			if id != expected {
				t.Fatalf("paging gap: id=%d want=%d", id, expected)
			}
			expected++
		}
		if _, ok := obj["_pp_truncated"]; ok {
			t.Fatal("private truncation field")
		}
		if n, ok := obj["count"].(float64); ok && n < 0 {
			t.Fatal("unknown count must be omitted")
		}
		if _, ok := obj["nextPageCursor"]; ok {
			t.Fatal("raw upstream cursor exposed")
		}
		cursor, _ = obj["next_cursor"].(string)
		if cursor == "" {
			t.Fatal("missing next cursor")
		}
	}
	if expected != 100 {
		t.Fatalf("only returned %d documents", expected)
	}
}
func TestE1TypedBudgetOverride(t *testing.T) {
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[{"id":0,"content":"`+strings.Repeat("x", 1100)+`"},{"id":1}],"nextPageCursor":null}`)
	})
	t.Setenv("PP_MCP_MAX_BYTES", "500")
	result, obj, text := e1Call(t, s, "documents_list", nil)
	if result.IsError || len(text) > 500 || obj["max_bytes"] != float64(500) {
		t.Fatalf("override ignored (%d bytes): %.200s", len(text), text)
	}
}
func TestE1ReadOnlyRegistration(t *testing.T) {
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	t.Setenv("READWISE_MCP_READ_ONLY", "1")
	// Re-register from scratch under the kill switch.
	s = server.NewMCPServer("Readwise", "test", server.WithToolCapabilities(false))
	RegisterTools(s)
	if s.GetTool("documents_list") == nil || s.GetTool("dedupe") == nil {
		t.Fatal("read tools missing")
	}
	for name, tool := range s.ListTools() {
		hint := tool.Tool.Annotations.ReadOnlyHint
		if hint == nil || !*hint {
			t.Fatalf("writable tool registered: %s", name)
		}
	}
}
func TestE1ReviewEmpty(t *testing.T) {
	detail := "Review not found."
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"detail": detail})
	})
	result, obj, text := e1Call(t, s, "review_get", nil)
	rows, ok := obj["highlights"].([]any)
	if result.IsError || obj["review_completed"] != false || obj["note"] != "no daily review available today" || !ok || len(rows) != 0 {
		t.Fatalf("no-review parity: %s", text)
	}
	detail = "Other not found."
	result, _, text = e1Call(t, s, "review_get", nil)
	if !result.IsError {
		t.Fatalf("other 404 accepted: %s", text)
	}
}
func TestE1BulkPartialFailure(t *testing.T) {
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		fmt.Fprint(w, `{"results":[{"id":"ok","success":true},{"id":"bad","success":false,"error":"fixture"},{"id":"also_ok","success":true}]}`)
	})
	result, obj, text := e1Call(t, s, "documents_bulk-update", map[string]any{"updates": []any{map[string]any{"id": "ok"}}})
	rows, ok := obj["results"].([]any)
	if result.IsError || obj["partial_failure"] != true || obj["failed_count"] != float64(1) || !ok || len(rows) != 3 {
		t.Fatalf("207 parity: %s", text)
	}
}
func TestE1MirrorWrapperDescription(t *testing.T) {
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	for name, tool := range s.ListTools() {
		if name == "read_next" {
			if !strings.Contains(tool.Tool.Description, "bare array") {
				t.Fatalf("wrong read-next output description: %s", tool.Tool.Description)
			}
			continue
		}
		if tool.Tool.Meta != nil && tool.Tool.Meta.AdditionalFields["pp:tenant-gate"] == "child-cli" && !strings.Contains(tool.Tool.Description, "{meta, results}") {
			t.Fatalf("mirror description missing wrapper: %s", name)
		}
	}
	if !strings.Contains(s.GetTool("reader-tags_list").Tool.Description, "Optional: cursor.") {
		t.Fatal("reader tags still documents wrong cursor")
	}
}

func TestFBulkPatchMCP(t *testing.T) {
	method := ""
	s := e1Server(t, func(w http.ResponseWriter, r *http.Request) { method = r.Method; fmt.Fprint(w, `{"results":[]}`) })
	result, _, text := e1Call(t, s, "documents_bulk-update", map[string]any{"updates": []any{map[string]any{"id": "d"}}})
	if result.IsError || method != "PATCH" {
		t.Fatalf("method=%s response=%s", method, text)
	}
}
