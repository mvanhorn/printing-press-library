// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestKlaviyoExecuteRequiresComplianceBeforePOST(t *testing.T) {
	for _, operation := range []struct{ id, path string }{
		{"campaign-send-jobs.send-campaign", "/api/campaign-send-jobs"},
		{"campaign-message-assign-template.assign-template-to-campaign-message", "/api/campaign-message-assign-template"},
		{"campaign-message-schedule.schedule-campaign-message", "/api/campaign-message-schedule"},
	} {
		for _, compliant := range []bool{false, true} {
			t.Run(operation.id+map[bool]string{false: "/noncompliant", true: "/compliant"}[compliant], func(t *testing.T) {
				t.Setenv(cliutil.MCPReadOnlyEnvVar, "")
				session := raMCPSession(t)
				posts, templateReads := 0, 0
				original := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = original })
				http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
					w := httptest.NewRecorder()
					w.Header().Set("Content-Type", "application/vnd.api+json")
					switch {
					case r.Method == http.MethodPost:
						posts++
						if r.URL.Path != operation.path || templateReads == 0 {
							t.Errorf("POST reached %s before compliance: template reads=%d", r.URL.Path, templateReads)
						}
						_, _ = io.WriteString(w, `{"data":{"id":"job-1"}}`)
					case r.URL.Path == "/api/campaigns/campaign-1":
						_, _ = io.WriteString(w, `{"data":{"id":"campaign-1","type":"campaign"}}`)
					case r.URL.Path == "/api/campaigns/message-1":
						w.WriteHeader(http.StatusNotFound)
						_, _ = io.WriteString(w, `{"errors":[{"detail":"not found"}]}`)
					case r.URL.Path == "/api/campaign-messages/message-1":
						_, _ = io.WriteString(w, `{"data":{"id":"message-1","relationships":{"campaign":{"data":{"id":"campaign-1"}}},"attributes":{"definition":{"status":"draft","send_options":{}}}}}`)
					case r.URL.Path == "/api/campaigns/campaign-1/campaign-messages":
						_, _ = io.WriteString(w, `{"data":[{"id":"message-1","attributes":{"definition":{"status":"draft","send_options":{}}}}],"links":{"next":null}}`)
					case r.URL.Path == "/api/campaign-messages/message-1/campaign-audience":
						_, _ = io.WriteString(w, `{"data":{"id":"audience-1","attributes":{"definition":{"included":[],"excluded":[]}}}}`)
					case r.URL.Path == "/api/campaign-messages/message-1/campaign-variations":
						_, _ = io.WriteString(w, `{"data":[{"id":"variation-1","attributes":{"definition":{"details":{"channel":"email","template_id":"template-1"}}}}],"links":{"next":null}}`)
					case r.URL.Path == "/api/templates/template-1":
						templateReads++
						html := "<p>Hello</p>"
						if compliant {
							html += `<a href="{% unsubscribe_link %}">Unsubscribe</a>`
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "template-1", "attributes": map[string]any{"html": html}}})
					default:
						t.Errorf("unexpected request: %s %s", r.Method, r.URL)
						w.WriteHeader(http.StatusNotFound)
					}
					return w.Result(), nil
				})
				id := "campaign-1"
				if operation.path == "/api/campaign-message-schedule" {
					id = "schedule-1"
				}
				result, err := handleCodeOrchExecute(platform.ContextWithSession(context.Background(), session), mcplib.CallToolRequest{
					Params: mcplib.CallToolParams{Arguments: map[string]any{
						"endpoint_id": operation.id, "confirm": true,
						"params": map[string]any{"data": map[string]any{
							"id": id, "relationships": map[string]any{
								"template":         map[string]any{"data": map[string]any{"id": "template-1"}},
								"campaign-message": map[string]any{"data": map[string]any{"id": "message-1"}},
							},
						}},
					}},
				})
				if err != nil || result == nil || len(result.Content) == 0 {
					t.Fatalf("execute result=%#v error=%v", result, err)
				}
				content, _ := mcplib.AsTextContent(result.Content[0])
				if compliant {
					if result.IsError || posts != 1 {
						t.Fatalf("compliant execute: error=%t posts=%d result=%s", result.IsError, posts, content.Text)
					}
				} else if !result.IsError || !strings.Contains(content.Text, "campaign compliance preflight failed") || posts != 0 {
					t.Fatalf("noncompliant execute: error=%t posts=%d result=%s", result.IsError, posts, content.Text)
				}
			})
		}
	}
}

func TestKlaviyoExecuteGuardRejectsMissingIDs(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/api/campaign-send-jobs", "campaign or message ID is required"},
		{"/api/campaign-message-assign-template", "template ID is missing"},
		{"/api/campaign-message-schedule", "campaign or message ID is required"},
	} {
		if err := requireKlaviyoExecuteCompliance(nil, tc.path, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s missing body error=%v", tc.path, err)
		}
	}
	if err := requireKlaviyoExecuteCompliance(nil, "/api/unrelated", []any{}); err != nil {
		t.Fatal(err)
	}
}

func TestKlaviyoScheduleExecutePreviewMakesNoRequests(t *testing.T) {
	t.Setenv(cliutil.MCPReadOnlyEnvVar, "")
	session := raMCPSession(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	requests := 0
	http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		return httptest.NewRecorder().Result(), nil
	})
	result, err := handleCodeOrchExecute(platform.ContextWithSession(context.Background(), session), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{Arguments: map[string]any{
			"endpoint_id": "campaign-message-schedule.schedule-campaign-message", "confirm": false,
			"params": map[string]any{"data": map[string]any{
				"type": "campaign-message-schedule", "attributes": map[string]any{"strategy": "immediate"},
				"relationships": map[string]any{"campaign-message": map[string]any{"data": map[string]any{"id": "message-1", "type": "campaign-message"}}},
			}},
		}},
	})
	if err != nil || result == nil || result.IsError || len(result.Content) == 0 || requests != 0 {
		t.Fatalf("preview result=%#v error=%v requests=%d", result, err, requests)
	}
	content, _ := mcplib.AsTextContent(result.Content[0])
	var preview struct{ Preview bool }
	if err := json.Unmarshal([]byte(content.Text), &preview); err != nil || !preview.Preview {
		t.Fatalf("expected preview, got %s: %v", content.Text, err)
	}
}
