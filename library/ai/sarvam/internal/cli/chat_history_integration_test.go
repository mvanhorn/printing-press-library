// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
