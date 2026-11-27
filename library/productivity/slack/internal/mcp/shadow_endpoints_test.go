// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestShadowEndpointsRegisteredInCodeOrch(t *testing.T) {
	wantIDs := []string{
		"bookmarks.list",
		"bookmarks.add",
		"bookmarks.edit",
		"bookmarks.remove",
		"search_api.files",
		"search_api.all",
		"auth_api.teams",
		"team.preferences",
		"team.billing",
		"web_client.unreads",
		"web_client.later",
		"web_client.drafts",
		"web_client.activity",
		"web_client.threads",
	}

	byID := make(map[string]bool, len(codeOrchEndpoints))
	for _, ep := range codeOrchEndpoints {
		byID[ep.ID] = true
	}
	for _, id := range wantIDs {
		if !byID[id] {
			t.Errorf("endpoint %q missing from codeOrchEndpoints", id)
		}
	}

	// Verify slack_search discovers newly registered endpoints.
	for _, query := range []string{"bookmarks channel", "search files", "unread counts", "saved later", "workspace billing"} {
		res, err := handleCodeOrchSearch(context.Background(), mcplib.CallToolRequest{
			Params: mcplib.CallToolParams{
				Name:      "slack_search",
				Arguments: map[string]any{"query": query, "limit": float64(5)},
			},
		})
		if err != nil {
			t.Fatalf("search %q returned err: %v", query, err)
		}
		if res == nil || res.IsError {
			t.Fatalf("search %q returned error result: %#v", query, res)
		}
		var payload struct {
			Count   int              `json:"count"`
			Results []map[string]any `json:"results"`
		}
		if err := json.Unmarshal([]byte(mcpTextContent(t, res)), &payload); err != nil {
			t.Fatalf("unmarshal search %q: %v", query, err)
		}
		if payload.Count == 0 {
			t.Errorf("search %q returned 0 results", query)
		}
	}
}

func TestCodeOrchExecute_SearchPrefersUserToken(t *testing.T) {
	var seenAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.URL.Path+"="+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"files":{"total":1,"matches":[{"id":"F1"}]}}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("SLACK_BASE_URL", srv.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-wrong-bot")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-right-user")

	for _, id := range []string{"search_api.files", "search_api.all"} {
		res, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{
			Params: mcplib.CallToolParams{
				Name: "slack_execute",
				Arguments: map[string]any{
					"endpoint_id": id,
					"params":      map[string]any{"query": "roadmap"},
				},
			},
		})
		if err != nil {
			t.Fatalf("%s handler error: %v", id, err)
		}
		if res == nil || res.IsError {
			t.Fatalf("%s returned error result: %#v", id, res)
		}
	}

	want := []string{
		"/search.files=Bearer xoxp-right-user",
		"/search.all=Bearer xoxp-right-user",
	}
	if len(seenAuth) != len(want) {
		t.Fatalf("seenAuth = %v, want %v", seenAuth, want)
	}
	for i := range want {
		if seenAuth[i] != want[i] {
			t.Errorf("seenAuth[%d] = %q, want %q", i, seenAuth[i], want[i])
		}
	}
}

func TestCodeOrchExecute_WebClientUsesSessionCredentials(t *testing.T) {
	var seenPath, seenAuth, seenCookie string
	var seenBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenCookie = r.Header.Get("Cookie")
		_ = json.NewDecoder(r.Body).Decode(&seenBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"channels":[]}`))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("SLACK_BASE_URL", srv.URL)
	t.Setenv("SLACK_BOT_TOKEN", "")
	t.Setenv("SLACK_USER_TOKEN", "")
	t.Setenv("SLACK_SESSION_TOKEN", "xoxc-mcp-session")
	t.Setenv("SLACK_COOKIE_D", "xoxd-mcp-cookie")

	res, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "slack_execute",
			Arguments: map[string]any{
				"endpoint_id": "web_client.unreads",
				"params":      map[string]any{},
			},
		},
	})
	if err != nil {
		t.Fatalf("web_client.unreads handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("web_client.unreads returned error result: %#v", res)
	}
	if seenPath != "/client.counts" {
		t.Errorf("path = %q, want /client.counts", seenPath)
	}
	if seenAuth != "Bearer xoxc-mcp-session" {
		t.Errorf("Authorization = %q, want Bearer xoxc-mcp-session", seenAuth)
	}
	if seenCookie != "d=xoxd-mcp-cookie" {
		t.Errorf("Cookie = %q, want d=xoxd-mcp-cookie", seenCookie)
	}
	if v, _ := seenBody["thread_counts_by_channel"].(bool); !v {
		t.Errorf("expected thread_counts_by_channel=true, got %v", seenBody)
	}

	// Verify ok:false invalid_auth returns a clear MCP error.
	authFailSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	}))
	defer authFailSrv.Close()
	t.Setenv("SLACK_BASE_URL", authFailSrv.URL)

	res, err = handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "slack_execute",
			Arguments: map[string]any{
				"endpoint_id": "web_client.later",
				"params":      map[string]any{"filter": "saved"},
			},
		},
	})
	if err != nil {
		t.Fatalf("web_client.later handler error: %v", err)
	}
	if res == nil || !res.IsError || !strings.Contains(mcpTextContent(t, res), "invalid_auth") {
		t.Fatalf("expected MCP error mentioning invalid_auth, got %#v", res)
	}
}

func TestCodeOrchExecute_SlackOkFalseReturnsMCPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/bookmarks.add":
			_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope","needed":"bookmarks:write"}`))
		case "/bookmarks.list":
			_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("SLACK_BASE_URL", srv.URL)
	t.Setenv("SLACK_USER_TOKEN", "xoxp-test-token")
	t.Setenv("SLACK_BOT_TOKEN", "")

	addRes, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "slack_execute",
			Arguments: map[string]any{
				"endpoint_id": "bookmarks.add",
				"params": map[string]any{
					"channel_id": "C123",
					"title":      "Runbook",
					"type":       "link",
					"link":       "https://example.com",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("bookmarks.add handler error: %v", err)
	}
	if addRes == nil || !addRes.IsError || !strings.Contains(mcpTextContent(t, addRes), "bookmarks:write") {
		t.Fatalf("expected MCP error mentioning bookmarks:write, got %#v", addRes)
	}

	listRes, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "slack_execute",
			Arguments: map[string]any{
				"endpoint_id": "bookmarks.list",
				"params":      map[string]any{"channel_id": "C999"},
			},
		},
	})
	if err != nil {
		t.Fatalf("bookmarks.list handler error: %v", err)
	}
	if listRes == nil || !listRes.IsError || !strings.Contains(mcpTextContent(t, listRes), "channel_not_found") {
		t.Fatalf("expected MCP error mentioning channel_not_found, got %#v", listRes)
	}
}
