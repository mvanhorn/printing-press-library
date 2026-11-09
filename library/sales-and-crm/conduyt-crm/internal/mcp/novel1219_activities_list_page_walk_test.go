// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// PATCH(novel1219-activities-list-page-number): Greptile PR #2450 thread 4514458610. GET /activities answers
// {"data":{"data":[...],"meta":{...,next_cursor}}} and takes the keyset cursor back through `cursor`. An agent that
// follows cursor_param from conduyt-crm_get with data.meta.next_cursor must walk a two-page fixture completely, and
// the keyset cursor must never reach `page` (the route parses page as an integer; a string there is a 500).
func TestNovel1219MCPActivitiesListFollowsNextCursor(t *testing.T) {
	ep := findCodeOrchEndpoint("activities.list")
	if ep == nil {
		t.Fatal("activities.list is not registered")
	}
	cursorParam, _ := codeOrchEndpointMetadata(ep)["cursor_param"].(string)

	const cursor1 = "WyIyMDI2LTExLTA5VDE3OjQwOjQ5LjI1NVoiLCJhY3QtcDEtYiJd"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if p := q.Get("page"); p != "" && p != "1" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"Internal server error"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if q.Get("cursor") == cursor1 {
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p2-c"}],"meta":{"page":1,"per_page":2,"total":3,"total_capped":false,"next_cursor":null}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p1-a"},{"id":"act-p1-b"}],"meta":{"page":1,"per_page":2,"total":3,"total_capped":false,"next_cursor":"`+cursor1+`"}}}`)
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))

	seen := map[string]bool{}
	params := map[string]any{"per_page": float64(2)}
	for call := 1; call <= 4; call++ {
		res, err := codeOrchTestInvoke(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
			Arguments: map[string]any{"endpoint_id": "activities.list", "params": params},
		}})
		if err != nil || res.IsError {
			t.Fatalf("call %d: err=%v result=%+v", call, err, res)
		}
		text := ""
		for _, c := range res.Content {
			if tc, ok := c.(mcplib.TextContent); ok {
				text += tc.Text
			}
		}
		var page struct {
			Data struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
				Meta struct {
					NextCursor *string `json:"next_cursor"`
				} `json:"meta"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(text), &page); err != nil {
			t.Fatalf("call %d: decode %q: %v", call, text, err)
		}
		for _, row := range page.Data.Data {
			seen[row.ID] = true
		}
		if page.Data.Meta.NextCursor == nil {
			break
		}
		params = map[string]any{"per_page": float64(2), cursorParam: *page.Data.Meta.NextCursor}
	}
	for _, id := range []string{"act-p1-a", "act-p1-b", "act-p2-c"} {
		if !seen[id] {
			t.Errorf("MCP walk following cursor_param %q missed %s (saw %v)", cursorParam, id, seen)
		}
	}
	if ep.Page.CursorParam != "cursor" || ep.Page.NextCursorPath != "data.meta.next_cursor" {
		t.Errorf("activities.list page config = %+v, want cursor via data.meta.next_cursor", ep.Page)
	}
}
