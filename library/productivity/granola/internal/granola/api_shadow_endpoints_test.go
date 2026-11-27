// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShadowDiscoveredInternalEndpoints(t *testing.T) {
	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-access-token")
	ResetTokenCache()
	t.Cleanup(ResetTokenCache)

	var seenPaths []string
	var seenWorkspaceHeaders []string
	var seenClientVersions []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		seenWorkspaceHeaders = append(seenWorkspaceHeaders, r.Header.Get("X-Granola-Workspace-Id"))
		seenClientVersions = append(seenClientVersions, r.Header.Get("X-Client-Version"))

		if r.Header.Get("Authorization") != "Bearer test-access-token" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == workspaceMembersEndpoint {
			body, _ := io.ReadAll(r.Body)
			var req map[string]string
			if err := json.Unmarshal(body, &req); err != nil || req["workspace_id"] != "ws-123" {
				http.Error(w, "bad workspace_id", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"members":[{"id":"u1"}],"invites":[]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case peopleEndpoint:
			_, _ = w.Write([]byte(`[{"id":"p1","name":"Alice"}]`))
		case preMeetingBriefsEndpoint:
			_, _ = w.Write([]byte(`{"briefs":[]}`))
		case followUpEmailsEndpoint:
			_, _ = w.Write([]byte(`{"drafts":[]}`))
		case mcpAgentsEndpoint:
			_, _ = w.Write([]byte(`{"agents":[],"connections":[]}`))
		case availableIntegrationsEndpoint:
			_, _ = w.Write([]byte(`{"integrations":[]}`))
		case workspacesNotesCountEndpoint:
			_, _ = w.Write([]byte(`{"total_notes_count":42,"workspaces":[]}`))
		case userInfoEndpoint:
			_, _ = w.Write([]byte(`{"id":"u1","workspace_ids":["ws-123"]}`))
		case subscriptionsEndpoint:
			_, _ = w.Write([]byte(`{"active_plan_id":"business"}`))
		case userSessionsEndpoint:
			_, _ = w.Write([]byte(`{"sessions":[]}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ic, err := NewInternalClient()
	if err != nil {
		t.Fatalf("NewInternalClient: %v", err)
	}
	ic.SetBaseURL(srv.URL)

	calls := []struct {
		name string
		fn   func() (json.RawMessage, error)
	}{
		{"GetUserInfo", ic.GetUserInfo},
		{"GetSubscriptions", ic.GetSubscriptions},
		{"ListUserSessions", ic.ListUserSessions},
		{"GetWorkspacesNotesCount", ic.GetWorkspacesNotesCount},
		{"GetWorkspaceMembers", func() (json.RawMessage, error) { return ic.GetWorkspaceMembers("ws-123") }},
		{"GetPeople", ic.GetPeople},
		{"GetPreMeetingBriefs", ic.GetPreMeetingBriefs},
		{"GetFollowUpEmails", ic.GetFollowUpEmails},
		{"GetMCPAgents", func() (json.RawMessage, error) { return ic.GetMCPAgents("ws-123") }},
		{"ListAvailableIntegrations", func() (json.RawMessage, error) { return ic.ListAvailableIntegrations("ws-123") }},
	}

	for _, tc := range calls {
		raw, err := tc.fn()
		if err != nil {
			t.Fatalf("%s failed: %v", tc.name, err)
		}
		if !json.Valid(raw) {
			t.Fatalf("%s returned invalid JSON: %s", tc.name, string(raw))
		}
	}

	if len(seenPaths) != len(calls) {
		t.Fatalf("expected %d requests, got %d", len(calls), len(seenPaths))
	}
	for i, ver := range seenClientVersions {
		if ver != granolaClientVersion {
			t.Errorf("call %d X-Client-Version = %q, want %q", i, ver, granolaClientVersion)
		}
	}
	// Calls 8 and 9 (GetMCPAgents and ListAvailableIntegrations) must carry X-Granola-Workspace-Id.
	if seenWorkspaceHeaders[8] != "ws-123" {
		t.Errorf("GetMCPAgents X-Granola-Workspace-Id = %q, want ws-123", seenWorkspaceHeaders[8])
	}
	if seenWorkspaceHeaders[9] != "ws-123" {
		t.Errorf("ListAvailableIntegrations X-Granola-Workspace-Id = %q, want ws-123", seenWorkspaceHeaders[9])
	}

	// Empty workspace ID should fail fast without hitting the network.
	if _, err := ic.GetWorkspaceMembers("   "); err == nil {
		t.Fatal("expected error for blank workspace_id")
	}
}

func TestShadowEndpointInvalidJSONRejected(t *testing.T) {
	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-access-token")
	ResetTokenCache()
	t.Cleanup(ResetTokenCache)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	ic, err := NewInternalClient()
	if err != nil {
		t.Fatalf("NewInternalClient: %v", err)
	}
	ic.SetBaseURL(srv.URL)

	if _, err := ic.GetPeople(); err == nil {
		t.Fatal("expected shapeError for non-JSON payload")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestShadowEndpointShardRouting(t *testing.T) {
	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-access-token")
	ResetTokenCache()
	t.Cleanup(ResetTokenCache)

	var gotURLs []string
	ic, err := NewInternalClient()
	if err != nil {
		t.Fatalf("NewInternalClient: %v", err)
	}
	// Intentionally keep default baseURL so postTo selects the real shard hosts,
	// intercepting at the HTTP transport layer with zero network IO.
	ic.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURLs = append(gotURLs, req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	})

	cases := []struct {
		name    string
		wantURL string
		fn      func() (json.RawMessage, error)
	}{
		{"GetUserInfo", "https://api.granola.ai/v1/get-user-info", ic.GetUserInfo},
		{"GetSubscriptions", "https://api.granola.ai/v1/get-subscriptions", ic.GetSubscriptions},
		{"ListUserSessions", "https://api.granola.ai/v1/list-user-sessions", ic.ListUserSessions},
		{"GetWorkspaceMembers", "https://api.granola.ai/v1/get-workspace-members", func() (json.RawMessage, error) { return ic.GetWorkspaceMembers("ws-123") }},
		{"GetWorkspacesNotesCount", "https://api.granola.ai/v1/get-workspaces-notes-count", ic.GetWorkspacesNotesCount},
		{"GetPeople", "https://pecan.api.granola.ai/v1/get-people", ic.GetPeople},
		{"GetPreMeetingBriefs", "https://maple.api.granola.ai/v1/get-pre-meeting-briefs", ic.GetPreMeetingBriefs},
		{"GetFollowUpEmails", "https://berry.api.granola.ai/v1/get-follow-up-emails", ic.GetFollowUpEmails},
		{"GetMCPAgents", "https://cinnamon.api.granola.ai/v1/get-mcp-agents", func() (json.RawMessage, error) { return ic.GetMCPAgents("ws-123") }},
		{"ListAvailableIntegrations", "https://chia.api.granola.ai/v1/list-available-integrations", func() (json.RawMessage, error) { return ic.ListAvailableIntegrations("ws-123") }},
	}

	for i, tc := range cases {
		if _, err := tc.fn(); err != nil {
			t.Fatalf("%s failed: %v", tc.name, err)
		}
		if gotURLs[i] != tc.wantURL {
			t.Errorf("%s routed to %q, want %q", tc.name, gotURLs[i], tc.wantURL)
		}
	}
}
