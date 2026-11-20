package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestSliceBReaderTagArraySchema(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("READWISE_API_TOKEN", "")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if want := []string{"alpha", "", "beta"}; !reflect.DeepEqual(r.URL.Query()["tag"], want) {
			t.Errorf("tags=%q want %q", r.URL.Query()["tag"], want)
		}
		fmt.Fprint(w, `{"results":[{"id":"fixture-doc","title":"kept"}],"nextPageCursor":null}`)
	}))
	defer srv.Close()
	t.Setenv("READWISE_BASE_URL", srv.URL)
	cfg := filepath.Join(home, "fixture.toml")
	if err := os.WriteFile(cfg, []byte("api_token = \"slice-b-fixture-token\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("READWISE_CONFIG", cfg)
	s := server.NewMCPServer("Readwise", "test", server.WithToolCapabilities(false))
	RegisterTools(s)
	tool := s.ListTools()["documents_list"]
	if tool == nil {
		t.Fatal("documents_list absent")
	}
	prop, ok := tool.Tool.InputSchema.Properties["tag"].(map[string]any)
	if !ok || prop["type"] != "array" {
		t.Fatalf("tag schema = %#v", tool.Tool.InputSchema.Properties["tag"])
	}
	req := mcplib.CallToolRequest{}
	req.Params.Name = "documents_list"
	req.Params.Arguments = map[string]any{"tag": []any{"alpha", "", "beta"}}
	result, err := tool.Handler(context.Background(), req)
	if err != nil || result.IsError || calls != 1 {
		t.Fatalf("MCP response=%+v err=%v calls=%d", result, err, calls)
	}
	req.Params.Arguments = map[string]any{"tag": []any{"1", "2", "3", "4", "5", "6"}}
	result, err = tool.Handler(context.Background(), req)
	if err != nil || !result.IsError || calls != 1 {
		t.Fatalf("MCP max tags response=%+v err=%v calls=%d", result, err, calls)
	}
}
