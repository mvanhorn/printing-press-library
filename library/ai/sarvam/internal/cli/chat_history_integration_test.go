// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/store"
)

func TestChatResumeSendsSavedFullConversation(t *testing.T) {
	var requests []struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		requests = append(requests, request)
		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{"id":"chat-first","choices":[{"message":{"role":"assistant","content":"First answer"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chat-second","choices":[{"message":{"role":"assistant","content":"Second answer"}}]}`))
	}))
	defer server.Close()

	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	configPath := filepath.Join(t.TempDir(), "missing.toml")
	for _, args := range [][]string{
		{"chat", "--messages", `[{"role":"system","content":"Keep this context"},{"role":"user","content":"First question"}]`, "--model", "sarvam-105b", "--config", configPath},
		{"chat", "resume", "chat-first", "Follow up", "--config", configPath},
	} {
		cmd := RootCmd()
		cmd.SetArgs(args)
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v", args[:2], err)
		}
	}
	if len(requests) != 2 || len(requests[1].Messages) != 4 {
		t.Fatalf("requests=%d, resumed messages=%d; want two requests and four resumed messages", len(requests), len(requests[1].Messages))
	}
	for i, want := range []string{"system:Keep this context", "user:First question", "assistant:First answer", "user:Follow up"} {
		got := requests[1].Messages[i].Role + ":" + requests[1].Messages[i].Content
		if got != want {
			t.Fatalf("resumed message %d = %q, want %q", i, got, want)
		}
	}
	db, err := store.OpenReadOnly(defaultDBPath("sarvam-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Get("chat", "chat-second"); err != nil {
		t.Fatalf("resumed conversation was not saved: %v", err)
	}
}

func TestChatStreamOutputsCompletedReplyAndSavesResumeContext(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var request struct {
			Stream   bool `json:"stream"`
			N        int  `json:"n"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || !request.Stream || request.N != 2 || len(request.Messages) != 1 || request.Messages[0].Content != "First question" {
			http.Error(w, "unexpected streamed chat request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(": stream opened\n\n" +
			"data: {\"id\":\"chat-stream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"First\"}}]}\n\n" +
			"data: {\"id\":\"chat-stream\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" answer\"}},{\"index\":1,\"delta\":{\"content\":\"Other answer\"}}]}\n\n" +
			"data: {\"id\":\"chat-stream\",\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"},{\"index\":1,\"finish_reason\":\"stop\"}],\"usage\":{\"total_tokens\":42}}\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer server.Close()

	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	cmd := RootCmd()
	cmd.SetArgs([]string{"chat", "--messages", `[{"role":"user","content":"First question"}]`, "--model", "sarvam-105b", "--stream", "--n", "2", "--config", filepath.Join(t.TempDir(), "missing.toml")})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chat --stream failed after a successful SSE response: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one streamed chat request", requests)
	}
	var printed struct {
		Results struct {
			ID      string `json:"id"`
			Stream  string `json:"stream"`
			Choices []struct {
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				TotalTokens int `json:"total_tokens"`
			} `json:"usage"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &printed); err != nil {
		t.Fatalf("stream output is not a valid JSON envelope: %v", err)
	}
	for _, want := range []string{"First", " answer", "Other answer", `"finish_reason":"stop"`, `"total_tokens":42`, "data: [DONE]"} {
		if !strings.Contains(printed.Results.Stream, want) {
			t.Fatalf("stream output omitted %q", want)
		}
	}
	if printed.Results.ID != "chat-stream" || len(printed.Results.Choices) != 2 || printed.Results.Choices[0].Message.Content != "First answer" || printed.Results.Choices[1].Message.Content != "Other answer" || printed.Results.Choices[1].FinishReason != "stop" || printed.Results.Usage.TotalTokens != 42 {
		t.Fatalf("stream output lost structured choices or metadata: %#v", printed.Results)
	}
	selected := RootCmd()
	selected.SetArgs([]string{"chat", "--messages", `[{"role":"user","content":"First question"}]`, "--model", "sarvam-105b", "--stream", "--n", "2", "--select", "id,choices", "--config", filepath.Join(t.TempDir(), "missing.toml")})
	var selectedOutput bytes.Buffer
	selected.SetOut(&selectedOutput)
	selected.SetErr(&selectedOutput)
	if err := selected.Execute(); err != nil {
		t.Fatalf("chat --stream --select failed: %v", err)
	}
	var projection struct {
		Results struct {
			ID      string            `json:"id"`
			Choices []json.RawMessage `json:"choices"`
		} `json:"results"`
	}
	if err := json.Unmarshal(selectedOutput.Bytes(), &projection); err != nil || projection.Results.ID != "chat-stream" || len(projection.Results.Choices) != 2 {
		t.Fatalf("selected stream fields missing: %s (error: %v)", selectedOutput.String(), err)
	}

	db, err := store.OpenReadOnly(defaultDBPath("sarvam-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw, err := db.Get("chat", "chat-stream")
	if err != nil {
		t.Fatalf("completed stream was not saved: %v", err)
	}
	record, err := decodeStoredChatConversation(raw)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := buildChatResumeMessages(record, "Follow up")
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 3 || resumed[1].(map[string]any)["content"] != "First answer" {
		t.Fatalf("saved stream cannot resume full context: %#v", resumed)
	}
}

func TestStreamedChatOutputLeavesToolCallsInRawStream(t *testing.T) {
	response := []byte("data: {\"id\":\"chat-tools\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n" +
		"data: {\"id\":\"chat-tools\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"lookup\"}}]}}]}\n\n" +
		"data: [DONE]\n\n")
	output, err := streamedChatOutput(response)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["choices"]; ok {
		t.Fatal("tool-call stream was presented as an incomplete structured choice")
	}
	if _, ok := result["id"]; ok {
		t.Fatal("tool-call stream was presented as a complete structured response")
	}
	var raw string
	if err := json.Unmarshal(result["stream"], &raw); err != nil || raw != string(response) {
		t.Fatalf("tool-call events were not preserved intact: %v", err)
	}
}
