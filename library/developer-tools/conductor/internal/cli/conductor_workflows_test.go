// Copyright 2026 Cole Grolmus and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/conductor/internal/config"
)

func workflowTestClient(server *httptest.Server) *client.Client {
	cfg := &config.Config{BaseURL: server.URL, ConductorApiKey: "test-key"}
	return client.New(cfg, time.Second, 0)
}

func TestValidateAgentModelEffort(t *testing.T) {
	tests := []struct {
		name                 string
		agent, model, effort string
		wantErr              bool
	}{
		{name: "codex high", agent: "codex", model: "gpt-5.4", effort: "high"},
		{name: "codex max needs 5.6", agent: "codex", model: "gpt-5.4", effort: "max", wantErr: true},
		{name: "codex ultra sol", agent: "codex", model: "gpt-5.6-sol", effort: "ultra"},
		{name: "codex ultra luna rejected", agent: "codex", model: "gpt-5.6-luna", effort: "ultra", wantErr: true},
		{name: "claude model on codex rejected", agent: "codex", model: "sonnet", effort: "high", wantErr: true},
		{name: "cursor effort rejected", agent: "cursor", model: "auto", effort: "high", wantErr: true},
		{name: "acp default", agent: "acp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentModelEffort(tt.agent, tt.model, tt.effort)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLaunchConductorCreatesWorkspaceThenSendsBrief(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "conductor-pp-cli/") {
			t.Errorf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/workspaces":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["repositoryUrl"] != "https://github.com/example/acme" {
				t.Errorf("repositoryUrl = %v", body["repositoryUrl"])
			}
			if body["agent"] != "codex" {
				t.Errorf("agent = %v", body["agent"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"workspaceId":"ws_1","sessionId":"sess_1","deepLink":"conductor://ws_1"}`))
		case "/v0/sessions/sess_1/messages":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["message"] != "Implement ENG-525" {
				t.Errorf("message = %v", body["message"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"messageId":"msg_1","state":"queued"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	receipt, err := launchConductor(context.Background(), workflowTestClient(server), launchOptions{
		RepositoryURL: "https://github.com/example/acme",
		Agent:         "codex", Model: "gpt-5.4", Effort: "high", Brief: "Implement ENG-525",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.WorkspaceID != "ws_1" || receipt.SessionID != "sess_1" || receipt.MessageID != "msg_1" {
		t.Fatalf("receipt = %+v", receipt)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(paths, ",") != "/v0/workspaces,/v0/sessions/sess_1/messages" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestMonitorDoesNotAcceptFalseIdleBeforeTranscriptChange(t *testing.T) {
	var mu sync.Mutex
	statusCalls := 0
	messageCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/sessions/sess_1/status":
			statusCalls++
			_, _ = w.Write([]byte(`{"workspaceId":"ws_1","sessionId":"sess_1","status":"idle","updatedAt":"2026-08-05T20:00:00Z"}`))
		case "/v0/sessions/sess_1/messages":
			messageCalls++
			if messageCalls == 1 {
				_, _ = w.Write([]byte(`{"data":[],"offset":0,"hasMore":false}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"id":"msg_2","sessionId":"sess_1","sessionIndex":2,"type":"assistant","content":"done","receivedAt":"2026-08-05T20:00:01Z"}],"offset":0,"hasMore":false}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	receipt, err := monitorConductor(context.Background(), workflowTestClient(server), "sess_1", "msg_1", time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Polls < 2 {
		t.Fatalf("polls = %d, false idle was accepted", receipt.Polls)
	}
	if receipt.CompletionProof != "transcript-change-then-idle" {
		t.Fatalf("proof = %q", receipt.CompletionProof)
	}
	if len(receipt.Events) != 1 || receipt.Events[0].ID != "msg_2" {
		t.Fatalf("events = %+v", receipt.Events)
	}
}

func TestMonitorResolvesQueueReceiptToTranscriptCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v0/sessions/sess_1/status":
			_, _ = w.Write([]byte(`{"workspaceId":"ws_1","sessionId":"sess_1","status":"idle"}`))
		case "/v0/sessions/sess_1/messages":
			if r.URL.Query().Get("after") == "queue_receipt_1" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Cursor message not found in this session"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"transcript_user_1","sessionId":"sess_1","sessionIndex":0,"type":"userMessage"},{"id":"agent_1","sessionId":"sess_1","sessionIndex":1,"type":"agent","content":"done"}],"offset":0,"hasMore":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	receipt, err := monitorConductor(context.Background(), workflowTestClient(server), "sess_1", "queue_receipt_1", time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.CompletionProof != "transcript-change-then-idle" {
		t.Fatalf("proof = %q", receipt.CompletionProof)
	}
	if len(receipt.Events) != 1 || receipt.Events[0].ID != "agent_1" {
		t.Fatalf("events = %+v", receipt.Events)
	}
}

func TestParseReportDuration(t *testing.T) {
	for input, want := range map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "1w": 7 * 24 * time.Hour} {
		got, err := parseReportDuration(input)
		if err != nil || got != want {
			t.Fatalf("parseReportDuration(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	if _, err := parseReportDuration("0d"); err == nil {
		t.Fatal("expected 0d to fail")
	}
}
