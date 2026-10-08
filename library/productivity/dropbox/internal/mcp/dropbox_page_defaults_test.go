package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

func listFolderHandler() func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	return makeAPIHandler("POST", "/files/list_folder", true, false, nil, mcpPageConfig{}, []mcpParamBinding{{PublicName: "path", WireName: "path", Location: "body"}, {PublicName: "limit", WireName: "limit", Location: "body"}}, []string{})
}

func callTool(t *testing.T, handler func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error), args map[string]any) string {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool error: %+v", result.Content)
	}
	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("content = %+v", result.Content)
	}
	return text.Text
}

func TestMCPListFolderInjectsDefaultPageSize(t *testing.T) {
	testenv.Isolate(t)
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"entries":[],"cursor":"c1","has_more":false}`)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	callTool(t, listFolderHandler(), map[string]any{"path": "/Folder"})
	callTool(t, listFolderHandler(), map[string]any{"path": "/Folder", "limit": 7})
	if len(bodies) != 2 {
		t.Fatalf("requests = %d", len(bodies))
	}
	if bodies[0]["limit"] != float64(mcpDefaultPageSizes["/files/list_folder"].size) {
		t.Fatalf("default page size not injected: %v", bodies[0])
	}
	if bodies[1]["limit"] != float64(7) {
		t.Fatalf("caller page size overridden: %v", bodies[1])
	}
}

func TestMCPLargeListFolderResumesWithNothingSkipped(t *testing.T) {
	testenv.Isolate(t)
	const total = 130
	pageSize := 2000 // Dropbox's own default when no limit is sent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Limit  int    `json:"limit"`
			Cursor string `json:"cursor"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		start := 0
		if r.URL.Path == "/files/list_folder/continue" {
			fmt.Sscanf(body.Cursor, "at-%d", &start)
		} else if body.Limit > 0 {
			// A real list_folder cursor keeps the first call's page size.
			pageSize = body.Limit
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		entries := make([]map[string]any, 0, end-start)
		for i := start; i < end; i++ {
			entries = append(entries, map[string]any{".tag": "file", "id": fmt.Sprintf("id:%d", i), "path_display": fmt.Sprintf("/Folder/sub/deeper/file-%04d.txt", i), "path_lower": fmt.Sprintf("/folder/sub/deeper/file-%04d.txt", i), "rev": "0123456789abcdef", "size": 1024, "content_hash": strings.Repeat("a", 64), "server_modified": "2026-01-01T00:00:00Z", "client_modified": "2026-01-01T00:00:00Z", "is_downloadable": true, "sharing_info": map[string]any{"read_only": false, "parent_shared_folder_id": "1234567890", "modified_by": "dbid:" + strings.Repeat("x", 40)}})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries, "cursor": fmt.Sprintf("at-%d", end), "has_more": end < total})
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	continueHandler := makeAPIHandler("POST", "/files/list_folder/continue", true, false, nil, mcpPageConfig{}, []mcpParamBinding{{PublicName: "cursor", WireName: "cursor", Location: "body"}}, []string{})
	seen := 0
	text := callTool(t, listFolderHandler(), map[string]any{"path": "/Folder"})
	for pages := 0; pages < 20; pages++ {
		var page struct {
			Entries        []map[string]any `json:"entries"`
			OmittedEntries []map[string]any `json:"omitted_entries"`
			Cursor         string           `json:"cursor"`
			HasMore        bool             `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(text), &page); err != nil {
			t.Fatalf("page %d: %v: %.200s", pages, err, text)
		}
		if len(page.OmittedEntries) != 0 || len(text) > 60000 {
			t.Fatalf("default page size still overflowed: %d bytes", len(text))
		}
		for _, entry := range page.Entries {
			if want := fmt.Sprintf("id:%d", seen); entry["id"] != want {
				t.Fatalf("entry %d id = %v, want %s", seen, entry["id"], want)
			}
			seen++
		}
		if !page.HasMore {
			break
		}
		text = callTool(t, continueHandler, map[string]any{"cursor": page.Cursor})
	}
	if seen != total {
		t.Fatalf("read %d of %d entries", seen, total)
	}
}

// A default page size is only safe when agents can follow the cursor, so
// every defaulted endpoint must have its continuation tool registered.
func TestMCPDefaultPageSizesOnlyForContinuableLists(t *testing.T) {
	continuation := map[string]string{
		"/files/list_folder":                   "/files/list_folder/continue",
		"/files/list_folder/get_latest_cursor": "/files/list_folder/continue",
		"/files/search_v2":                     "/files/search/continue_v2",
		"/sharing/list_folders":                "/sharing/list_folders/continue",
		"/file_requests/list_v2":               "/file_requests/list/continue",
	}
	tools, err := os.ReadFile("tools.go")
	if err != nil {
		t.Fatal(err)
	}
	for path := range mcpDefaultPageSizes {
		next, ok := continuation[path]
		if !ok {
			t.Fatalf("%s has a default page size but no known continuation endpoint", path)
		}
		if !strings.Contains(string(tools), fmt.Sprintf("%q", next)) {
			t.Fatalf("%s has a default page size but %s is not an MCP tool", path, next)
		}
	}
	for _, path := range []string{"/sharing/list_folder_members", "/sharing/list_received_files"} {
		args := withMCPDefaultPageSize(path, map[string]any{})
		if len(args) != 0 {
			t.Fatalf("%s got a default page size without a continuation tool: %v", path, args)
		}
	}
}
