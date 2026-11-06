// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestThinExecuteCannotBypassChatConsent(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer s.Close()
	t.Setenv("TELLA_BASE_URL", s.URL)
	t.Setenv("TELLA_API_KEY", "fixture")
	for _, confirm := range []bool{false, true} {
		var req mcplib.CallToolRequest
		req.Params.Arguments = map[string]any{"endpoint_id": "agent-chats.start", "confirm": confirm, "params": map[string]any{"prompt": "review", "videoId": "v"}}
		result, err := handleCodeOrchExecute(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		if confirm && !result.IsError {
			t.Fatal("thin bypass allowed")
		}
		if !confirm && result.IsError {
			t.Fatal("dry preview refused")
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
}
func TestNewWorkflowMCPToolsAndConsentSchema(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	seen := map[string]bool{}
	for _, entry := range s.ListTools() {
		if entry.Tool.Meta == nil {
			continue
		}
		command, _ := entry.Tool.Meta.AdditionalFields["pp:cli-command"].(string)
		seen[command] = true
		if command == "agent-chats start" {
			if _, ok := entry.Tool.InputSchema.Properties["yes"]; !ok {
				t.Fatal("missing explicit yes schema")
			}
			if !strings.Contains(entry.Tool.Name, "agent_chats_start") {
				t.Fatal(entry.Tool.Name)
			}
		}
	}
	for _, name := range []string{"videos preview", "videos auto", "clips find-retakes", "lessons pass", "lessons ship", "videos storyboard-image", "agent-chats start"} {
		if !seen[name] {
			t.Error("missing " + name)
		}
	}
}

func TestCreatingPreviewCommandsAreNotReadOnly(t *testing.T) {
	for _, path := range [][]string{{"videos", "preview"}, {"videos", "live-preview", "get-link"}, {"videos", "recording-link", "get"}} {
		cmd, remaining, err := cli.RootCmd().Find(path)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("command %v missing: %v", path, err)
		}
		if cmd.Annotations["mcp:read-only"] != "false" {
			t.Fatalf("creating-preview command %v marked read-only", path)
		}
	}
	for _, readOnly := range []string{"0", "1"} {
		t.Setenv("TELLA_MCP_READ_ONLY", readOnly)
		s := server.NewMCPServer("test", "test")
		RegisterTools(s)
		previewPresent := false
		for _, entry := range s.ListTools() {
			if entry.Tool.Meta == nil {
				continue
			}
			command, _ := entry.Tool.Meta.AdditionalFields["pp:cli-command"].(string)
			if command == "videos preview" {
				previewPresent = true
			}
			if readOnly == "1" && (command == "videos preview" || command == "videos live-preview get-link" || command == "videos recording-link get") {
				t.Fatalf("read-only MCP registered %s", command)
			}
		}
		if readOnly == "0" && !previewPresent {
			t.Fatal("ordinary MCP lost videos preview")
		}
	}
}

func TestThinLinkCreationRequiresConfirmation(t *testing.T) {
	for _, endpoint := range []struct{ id, path string }{
		{"videos.live-preview.get-link", "/v1/videos/vid_fixture/live-preview"},
		{"videos.recording-link.get", "/v1/videos/vid_fixture/recording-link"},
	} {
		for _, tc := range []struct {
			name                                      string
			readOnly, execute, confirm, refused, post bool
		}{
			{"read", false, false, false, true, false},
			{"read-only read", true, false, false, true, false},
			{"preview", false, true, false, false, false},
			{"confirmed", false, true, true, false, true},
			{"read-only preview", true, true, false, true, false},
			{"read-only confirmed", true, true, true, true, false},
		} {
			t.Run(endpoint.id+"/"+tc.name, func(t *testing.T) {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != http.MethodPost || r.URL.Path != endpoint.path || r.URL.RawQuery != "" {
						t.Errorf("request = %s %s, want POST %s", r.Method, r.URL, endpoint.path)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"url":"https://example.invalid/fixture#bearer"}`))
				}))
				defer s.Close()
				t.Setenv("TELLA_BASE_URL", s.URL)
				t.Setenv("TELLA_API_KEY", "fixture")
				t.Setenv(cliutil.MCPReadOnlyEnvVar, "0")
				if tc.readOnly {
					t.Setenv(cliutil.MCPReadOnlyEnvVar, "1")
				}
				var req mcplib.CallToolRequest
				req.Params.Arguments = map[string]any{"endpoint_id": endpoint.id, "params": map[string]any{"id": "vid_fixture"}}
				if tc.confirm {
					req.Params.Arguments.(map[string]any)["confirm"] = true
				}
				handler := handleCodeOrchRead
				if tc.execute {
					handler = handleCodeOrchExecute
				}
				result, err := handler(t.Context(), req)
				if err != nil || result == nil || result.IsError != tc.refused {
					t.Fatalf("result = %#v, err = %v; want refusal %v", result, err, tc.refused)
				}
				text := mcpTextContent(t, result)
				if tc.refused {
					want := "tella_execute"
					if tc.readOnly {
						want = cliutil.MCPReadOnlyEnvVar
					}
					if !strings.Contains(text, want) {
						t.Fatalf("refusal = %s, want %s", text, want)
					}
				} else if !tc.post {
					var preview map[string]any
					if err := json.Unmarshal([]byte(text), &preview); err != nil {
						t.Fatal(err)
					}
					if preview["preview"] != true || preview["confirmation_required"] != true || preview["method"] != "POST" || preview["path"] != endpoint.path || preview["endpoint_id"] != endpoint.id {
						t.Fatalf("incorrect confirmation preview: %s", text)
					}
				}
				wantCalls := 0
				if tc.post {
					wantCalls = 1
				}
				if calls != wantCalls {
					t.Fatalf("HTTP calls = %d, want %d", calls, wantCalls)
				}
			})
		}
	}
}
