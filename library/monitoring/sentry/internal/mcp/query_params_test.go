// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

type seenRequest struct {
	method string
	query  map[string][]string
	body   map[string]any
}

func callAPITool(t *testing.T, method, path string, positional []string, args map[string]any, queryParams ...string) seenRequest {
	t.Helper()
	var seen seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.method = r.Method
		seen.query = r.URL.Query()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &seen.body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SENTRY_AUTH_TOKEN", "test-token")
	t.Setenv("SENTRY_BASE_URL", srv.URL)
	handler := makeAPIHandler(method, path, positional, queryParams...)
	req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "test", Arguments: args}}
	result, err := handler(context.Background(), req)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("tool call failed: err=%v result=%+v", err, result)
	}
	return seen
}

func TestMCPBulkDeleteSendsDeclaredQueryParams(t *testing.T) {
	seen := callAPITool(t, "DELETE", "/api/0/organizations/{organization_id_or_slug}/issues/", []string{"organization_id_or_slug"},
		map[string]any{"organization_id_or_slug": "test-org", "id": []any{float64(10), float64(4512244350713856)}, "query": "is:resolved"},
		"id", "query", "limit")
	if seen.method != http.MethodDelete {
		t.Fatalf("method = %s, want DELETE", seen.method)
	}
	if !reflect.DeepEqual(seen.query["id"], []string{"10", "4512244350713856"}) || !reflect.DeepEqual(seen.query["query"], []string{"is:resolved"}) {
		t.Errorf("query = %v, want id=10&id=4512244350713856&query=is:resolved", seen.query)
	}
}

func TestMCPBulkMutateKeepsQueryParamsOutOfTheBody(t *testing.T) {
	seen := callAPITool(t, "PUT", "/api/0/projects/{organization_id_or_slug}/{project_id_or_slug}/issues/",
		[]string{"organization_id_or_slug", "project_id_or_slug"},
		map[string]any{"organization_id_or_slug": "o", "project_id_or_slug": "p", "id": "7", "status": "resolved", "isBookmarked": true},
		"id", "status")
	if !reflect.DeepEqual(seen.query["id"], []string{"7"}) || !reflect.DeepEqual(seen.query["status"], []string{"resolved"}) {
		t.Errorf("query = %v, want id=7&status=resolved", seen.query)
	}
	if _, ok := seen.body["id"]; ok {
		t.Errorf("query param id also sent in body: %v", seen.body)
	}
	if seen.body["isBookmarked"] != true {
		t.Errorf("body lost isBookmarked: %v", seen.body)
	}
}

func TestMCPGetRepeatsNativeArrays(t *testing.T) {
	seen := callAPITool(t, "GET", "/api/0/organizations/{organization_id_or_slug}/events/", []string{"organization_id_or_slug"},
		map[string]any{"organization_id_or_slug": "o", "field": []any{"project", "count_if(a,equals,b)"}, "dataset": "errors"})
	if !reflect.DeepEqual(seen.query["field"], []string{"project", "count_if(a,equals,b)"}) || !reflect.DeepEqual(seen.query["dataset"], []string{"errors"}) {
		t.Errorf("query = %v, want field=project&field=count_if(a,equals,b)&dataset=errors", seen.query)
	}
}

func TestMCPMutationsPreserveExplicitEmptyQuery(t *testing.T) {
	for _, method := range []string{"DELETE", "PUT"} {
		seen := callAPITool(t, method, "/api/0/organizations/{organization_id_or_slug}/issues/",
			[]string{"organization_id_or_slug"}, map[string]any{"organization_id_or_slug": "o", "query": ""}, "query")
		if !reflect.DeepEqual(seen.query["query"], []string{""}) {
			t.Errorf("%s query = %v, want query=", method, seen.query)
		}
	}
}
