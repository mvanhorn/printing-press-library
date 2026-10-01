// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConversationsHistoryUserOpensDMWithUserToken(t *testing.T) {
	var opened, fetched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xoxp-user-token-placeholder" {
			t.Errorf("Authorization = %q, want user token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.open":
			opened = true
			if r.Method != http.MethodPost {
				t.Errorf("open method = %s, want POST", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode open body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if body["users"] != "U123" {
				t.Errorf("open users = %#v, want U123", body["users"])
			}
			if body["prevent_creation"] != true {
				t.Errorf("open prevent_creation = %#v, want true", body["prevent_creation"])
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"D456"}}`))
		case "/conversations.history":
			fetched = true
			if r.Method != http.MethodGet {
				t.Errorf("history method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("channel"); got != "D456" {
				t.Errorf("history channel = %q, want D456", got)
			}
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"ts":"1","text":"hello"}],"has_more":false}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")
	t.Setenv("SLACK_DATA_DIR", t.TempDir())

	flags := &rootFlags{asJSON: true}
	cmd := newConversationsHistoryCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--user", "U123", "--limit", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute history --user: %v", err)
	}
	if !opened || !fetched {
		t.Fatalf("opened/fetched = %v/%v, want true/true", opened, fetched)
	}
	var output struct {
		Results []struct {
			Text string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if len(output.Results) != 1 || output.Results[0].Text != "hello" {
		t.Fatalf("results = %#v, want one hello message", output.Results)
	}
}

func TestConversationsHistoryRejectsChannelAndUserTogether(t *testing.T) {
	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--channel", "D456", "--user", "U123"})

	err := cmd.Execute()
	if err == nil || err.Error() != "--channel and --user are mutually exclusive; provide one or the other" {
		t.Fatalf("error = %v, want mutual-exclusion error", err)
	}
}

func TestConversationsHistoryDMWithBothTokensTriesBotFirst(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/conversations.history" || r.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer xoxb-bot-token-placeholder" {
			t.Errorf("Authorization = %q, want bot token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"ts":"1","text":"hello"}]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")
	t.Setenv("SLACK_DATA_DIR", t.TempDir())

	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--channel", "D456"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want one history read", requests)
	}
}

func TestConversationsHistoryDMDryRunNeedsNoTokenOrRequest(t *testing.T) {
	t.Setenv("SLACK_USER_TOKEN", "")
	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true, dryRun: true})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--user", "U123", "--limit", "10", "--cursor", "next", "--oldest", "1", "--latest", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		DryRun         bool              `json:"dry_run"`
		ExistingDMOnly bool              `json:"existing_dm_only"`
		Query          map[string]string `json:"query"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || !result.ExistingDMOnly {
		t.Fatalf("dry-run result = %+v", result)
	}
	for key, want := range map[string]string{"limit": "10", "cursor": "next", "oldest": "1", "latest": "2"} {
		if result.Query[key] != want {
			t.Fatalf("dry-run query %s = %q, want %q", key, result.Query[key], want)
		}
	}
}

func TestConversationsHistoryDMRejectsLocalSourceBeforeNetwork(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")

	for _, args := range [][]string{{"--user", "U123"}, {"--channel", "D456"}} {
		cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true, dataSource: "local"})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "no local data source") {
			t.Fatalf("history %v error = %v, want unsupported local source", args, err)
		}
	}
	if requests != 0 {
		t.Fatalf("local source made %d network requests, want none", requests)
	}
}

func TestConversationsHistoryDMFallsBackAfterBotChannelNotFound(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		auth := r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/conversations.history":
			channel := r.URL.Query().Get("channel")
			if channel == "D456" {
				_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
				return
			}
			if channel != "D789" || auth != "Bearer xoxp-user-token-placeholder" {
				t.Errorf("unexpected resolved history request: channel=%s auth=%s", channel, auth)
			}
			_, _ = w.Write([]byte(`{"ok":true,"messages":[{"ts":"1","text":"hello"}]}`))
		case "/conversations.info":
			if auth != "Bearer xoxp-user-token-placeholder" {
				t.Errorf("peer lookup auth = %s, want user token", auth)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"user":"U123"}}`))
		case "/conversations.open":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["prevent_creation"] != true {
				t.Errorf("DM resolution must use prevent_creation: body=%v error=%v", body, err)
			}
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"D789"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")
	t.Setenv("SLACK_DATA_DIR", t.TempDir())

	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--channel", "D456"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 5 || !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("fallback requests = %d, output = %q; want five requests and the resolved DM", requests, stdout.String())
	}
}

func TestConversationsHistoryDMFallsBackAfterBotMissingScope(t *testing.T) {
	var auths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.history" || r.URL.Query().Get("channel") != "D456" {
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		auth := r.Header.Get("Authorization")
		auths = append(auths, auth)
		w.Header().Set("Content-Type", "application/json")
		if auth == "Bearer xoxb-bot-token-placeholder" {
			_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope","needed":"im:history"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"ts":"1","text":"hello"}]}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")
	t.Setenv("SLACK_DATA_DIR", t.TempDir())

	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--channel", "D456"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(auths) != 2 || auths[0] != "Bearer xoxb-bot-token-placeholder" || auths[1] != "Bearer xoxp-user-token-placeholder" {
		t.Fatalf("auth order = %v, want bot then user", auths)
	}
	if !strings.Contains(stdout.String(), "hello") {
		t.Fatalf("history output = %q, want user-token DM result", stdout.String())
	}
}

func TestConversationsHistoryDMReportsRetryScopeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/conversations.history":
			if r.URL.Query().Get("channel") == "D789" {
				_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope","needed":"im:history"}`))
			} else {
				_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
			}
		case "/conversations.info":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"user":"U123"}}`))
		case "/conversations.open":
			_, _ = w.Write([]byte(`{"ok":true,"channel":{"id":"D789"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SLACK_BASE_URL", server.URL)
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-token-placeholder")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-placeholder")
	t.Setenv("SLACK_DATA_DIR", t.TempDir())

	cmd := newConversationsHistoryCmd(&rootFlags{asJSON: true})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--channel", "D456"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "missing Slack scope: im:history") {
		t.Fatalf("retry error = %v, want required Slack scope", err)
	}
}
