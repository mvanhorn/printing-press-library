// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand patch: see .printing-press-patches/review-mcp-safety.json.

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// codeOrchShapeIsReadOnly reports whether an endpoint record is structurally a
// read: a GET lookup, or a POST aimed exactly at the read-only
// /data/user-data/get data-fetch route.
func codeOrchShapeIsReadOnly(ep *codeOrchEndpoint) bool {
	if ep.Method == "GET" {
		// recipes.link is a local-cache command, not a remote endpoint;
		// generic execute must not send its sentinel path through HTTP.
		return ep.Path != "local-cache"
	}
	return ep.Method == "POST" && ep.Path == "/data/user-data/get"
}

func codeOrchRequest(name string, args map[string]any) mcplib.CallToolRequest {
	var req mcplib.CallToolRequest
	req.Params.Name = name
	req.Params.Arguments = args
	return req
}

func toolResultText(t *testing.T, res *mcplib.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatalf("empty result content")
	}
	tc, ok := res.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", res.Content[0])
	}
	return tc.Text
}

// The allowlist must track endpoint shapes exactly: nothing structurally
// read-only may be left out (backward compatibility) and nothing shaped like a
// mutation may be in (the whole point of the gate).
func TestCodeOrchReadOnlyAllowlistTracksEndpointShape(t *testing.T) {
	seen := map[string]bool{}
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		if seen[ep.ID] {
			t.Fatalf("duplicate endpoint id %q", ep.ID)
		}
		seen[ep.ID] = true
		readOnlyShape := codeOrchShapeIsReadOnly(ep)
		classified := codeOrchReadOnlyEndpoints[ep.ID]
		if classified != readOnlyShape {
			t.Errorf("endpoint %q (%s %s): classified read-only=%v but shape says %v", ep.ID, ep.Method, ep.Path, classified, readOnlyShape)
		}
	}
}

// Mutation requests must be rejected by the gate itself — the HOME dir is
// empty so config loading cannot shape the error, and the gate runs before
// client construction, so no HTTP request is ever attempted.
func TestCodeOrchExecuteRejectsMutationsBeforeClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mutations := []string{
		"categories.create", "categories.rename", "categories.delete", "categories.reorder",
		"items.add", "items.remove", "items.update", "items.check",
		"lists.create", "lists.delete", "lists.rename", "lists.reset", "lists.settings",
		"meal.add", "meal.update", "meal.delete",
		"recipes.create", "recipes.delete", "recipes.batch-add",
		"collections.add", "collections.delete",
		"starters.remove", "favorites.add", "folders.update", "recipes.link",
	}
	for _, id := range mutations {
		res, err := handleCodeOrchExecute(context.Background(), codeOrchRequest("anylist_execute", map[string]any{"endpoint_id": id}))
		if err != nil {
			t.Fatalf("%s: handler error: %v", id, err)
		}
		if !res.IsError {
			t.Fatalf("%s: expected an error result, got: %s", id, toolResultText(t, res))
		}
		text := toolResultText(t, res)
		if !strings.Contains(text, "anylist-pp-cli") {
			t.Errorf("%s: rejection does not direct the caller to the CLI mirror: %s", id, text)
		}
		if !strings.Contains(text, "read-only") {
			t.Errorf("%s: rejection does not state the read-only boundary: %s", id, text)
		}
	}
}

// Search must never return a non-read-only endpoint_id, no matter how the
// mutation is phrased in the query.
func TestCodeOrchSearchHidesMutationRoutes(t *testing.T) {
	for _, q := range []string{"add", "delete", "remove", "update", "create", "check", "reset", "sharing", "settings"} {
		res, err := handleCodeOrchSearch(context.Background(), codeOrchRequest("anylist_search", map[string]any{"query": q, "limit": 100}))
		if err != nil {
			t.Fatalf("query %q: handler error: %v", q, err)
		}
		var parsed struct {
			Count   int              `json:"count"`
			Results []map[string]any `json:"results"`
		}
		if err := json.Unmarshal([]byte(toolResultText(t, res)), &parsed); err != nil {
			t.Fatalf("query %q: unmarshaling results: %v", q, err)
		}
		if len(parsed.Results) != parsed.Count {
			t.Errorf("query %q: count %d != len(results) %d", q, parsed.Count, len(parsed.Results))
		}
		for _, r := range parsed.Results {
			id, _ := r["endpoint_id"].(string)
			if !codeOrchReadOnlyEndpoints[id] {
				t.Errorf("query %q returned non-read-only endpoint %q", q, id)
			}
		}
	}
}

// Representative read-only reads must remain searchable and must pass the
// execute gate (admitted for execution; no live request is sent here).
func TestCodeOrchReadOnlyEndpointsStayAvailable(t *testing.T) {
	byID := func(id string) *codeOrchEndpoint {
		for i := range codeOrchEndpoints {
			if codeOrchEndpoints[i].ID == id {
				return &codeOrchEndpoints[i]
			}
		}
		t.Fatalf("endpoint %q not found in registry", id)
		return nil
	}

	readOnlyIDs := []string{
		"export", "collections.list", "recipes.ingredients",
		"items.list", "items.lookup", "meal.summary", "recipes.search", "stores.list",
	}
	for _, id := range readOnlyIDs {
		ep := byID(id)
		if err := codeOrchReadOnlyGate(ep); err != nil {
			t.Errorf("read-only endpoint %q failed the execute gate: %v", id, err)
		}
	}

	expectHit := map[string]string{
		"export":      "export",
		"barcode":     "items.lookup",
		"ingredients": "recipes.ingredients",
		"collections": "collections.list",
	}
	for q, wantID := range expectHit {
		res, err := handleCodeOrchSearch(context.Background(), codeOrchRequest("anylist_search", map[string]any{"query": q, "limit": 100}))
		if err != nil {
			t.Fatalf("query %q: handler error: %v", q, err)
		}
		var parsed struct {
			Results []map[string]any `json:"results"`
		}
		if err := json.Unmarshal([]byte(toolResultText(t, res)), &parsed); err != nil {
			t.Fatalf("query %q: unmarshaling results: %v", q, err)
		}
		found := false
		for _, r := range parsed.Results {
			if r["endpoint_id"] == wantID {
				found = true
			}
		}
		if !found {
			t.Errorf("query %q did not surface read-only endpoint %q (results: %d)", q, wantID, len(parsed.Results))
		}
	}
}
