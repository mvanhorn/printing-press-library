// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupSlackShadowTestEnv(t *testing.T, serverURL string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("SLACK_BASE_URL", serverURL)
	t.Setenv("SLACK_USER_TOKEN", "xoxp-test-token")
	t.Setenv("SLACK_BOT_TOKEN", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
}

func TestSlackShadowEndpoints_OAuthCommands(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer xoxp-test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search.files":
			if r.URL.Query().Get("query") != "roadmap" {
				http.Error(w, "missing query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"query":"roadmap","files":{"total":1,"matches":[{"id":"F1"}]}}`))
		case "/search.all":
			_, _ = w.Write([]byte(`{"ok":true,"query":"roadmap","messages":{"total":1,"matches":[]},"files":{"total":0,"matches":[]}}`))
		case "/auth.teams.list":
			_, _ = w.Write([]byte(`{"ok":true,"teams":[{"id":"T1","name":"Acme"}]}`))
		case "/team.preferences.list":
			_, _ = w.Write([]byte(`{"ok":true,"allow_message_deletion":true}`))
		case "/team.billing.info":
			_, _ = w.Write([]byte(`{"ok":true,"plan":"pro"}`))
		case "/bookmarks.list":
			if r.URL.Query().Get("channel_id") != "C123" {
				http.Error(w, "missing channel_id", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"bookmarks":[{"id":"Bk1","title":"Runbook"}]}`))
		case "/bookmarks.add":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"channel_id":"C123"`) {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"bookmark":{"id":"Bk2","title":"Docs"}}`))
		case "/bookmarks.edit":
			_, _ = w.Write([]byte(`{"ok":true,"bookmark":{"id":"Bk2","title":"Docs v2"}}`))
		case "/bookmarks.remove":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()
	setupSlackShadowTestEnv(t, srv.URL)

	cases := [][]string{
		{"search-api", "files", "--query", "roadmap", "--json"},
		{"search-api", "all", "--query", "roadmap", "--json"},
		{"auth-api", "teams", "--json"},
		{"team", "preferences", "--json"},
		{"team", "billing", "--json"},
		{"bookmarks", "list", "--channel", "C123", "--json"},
		{"bookmarks", "add", "--channel", "C123", "--title", "Docs", "--link", "https://example.com", "--json"},
		{"bookmarks", "edit", "--channel", "C123", "--bookmark-id", "Bk2", "--title", "Docs v2", "--json"},
		{"bookmarks", "remove", "--channel", "C123", "--bookmark-id", "Bk2", "--json"},
	}

	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		cmd := newRootCmd(&rootFlags{})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(append(args, "--no-learn", "--no-cache"))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v (stderr=%s)", args, err, stderr.String())
		}
		if !json.Valid(stdout.Bytes()) {
			t.Fatalf("%v produced invalid JSON: %s", args, stdout.String())
		}
	}

	if len(seen) != len(cases) {
		t.Fatalf("expected %d requests, got %d (%v)", len(cases), len(seen), seen)
	}
}

func TestSlackShadowEndpoints_OkFalseSurfacesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"missing_scope","needed":"bookmarks:read"}`))
	}))
	defer srv.Close()
	setupSlackShadowTestEnv(t, srv.URL)

	var stdout, stderr bytes.Buffer
	cmd := newRootCmd(&rootFlags{})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"bookmarks", "list", "--channel", "C123", "--json", "--no-learn", "--no-cache"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ok:false Slack response")
	}
}

func TestSlackWebClient_AuthAndEndpoints(t *testing.T) {
	var seenPaths []string
	var seenCookie, seenAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		seenAuth = r.Header.Get("Authorization")
		seenCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"items":[]}`))
	}))
	defer srv.Close()
	setupSlackShadowTestEnv(t, srv.URL)

	// Missing SLACK_SESSION_TOKEN must return exit code 4 (authErr).
	t.Setenv("SLACK_SESSION_TOKEN", "")
	t.Setenv("SLACK_COOKIE_D", "")
	{
		cmd := newRootCmd(&rootFlags{})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"web-client", "unreads", "--json", "--no-learn"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected auth error when SLACK_SESSION_TOKEN is missing")
		}
		if ExitCode(err) != 4 {
			t.Fatalf("ExitCode = %d, want 4 (authErr)", ExitCode(err))
		}
	}

	// Dry-run works without credentials or network.
	{
		var out bytes.Buffer
		cmd := newRootCmd(&rootFlags{})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"web-client", "later", "--dry-run", "--json", "--no-learn"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("web-client later --dry-run failed: %v", err)
		}
		if !strings.Contains(out.String(), `"dry_run": true`) && !strings.Contains(out.String(), `"dry_run":true`) {
			t.Fatalf("unexpected dry-run output: %s", out.String())
		}
	}

	// Valid xoxc + d cookie hits all 5 discovered web-client endpoints cleanly.
	t.Setenv("SLACK_SESSION_TOKEN", "xoxc-test-session")
	t.Setenv("SLACK_COOKIE_D", "xoxd-test-cookie")

	subcmds := [][]string{
		{"web-client", "unreads", "--json", "--no-learn"},
		{"web-client", "later", "--filter", "saved", "--limit", "10", "--json", "--no-learn"},
		{"web-client", "drafts", "--limit", "10", "--json", "--no-learn"},
		{"web-client", "activity", "--limit", "10", "--json", "--no-learn"},
		{"web-client", "threads", "--limit", "10", "--json", "--no-learn"},
	}
	for _, args := range subcmds {
		var out bytes.Buffer
		cmd := newRootCmd(&rootFlags{})
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v", args, err)
		}
	}

	if seenAuth != "Bearer xoxc-test-session" {
		t.Errorf("Authorization = %q, want Bearer xoxc-test-session", seenAuth)
	}
	if seenCookie != "d=xoxd-test-cookie" {
		t.Errorf("Cookie = %q, want d=xoxd-test-cookie", seenCookie)
	}
	wantPaths := []string{
		"/client.counts",
		"/saved.list",
		"/drafts.list",
		"/activity.feed",
		"/subscriptions.thread.getView",
	}
	if len(seenPaths) != len(wantPaths) {
		t.Fatalf("seenPaths = %v, want %v", seenPaths, wantPaths)
	}
	for i, want := range wantPaths {
		if seenPaths[i] != want {
			t.Errorf("seenPaths[%d] = %q, want %q", i, seenPaths[i], want)
		}
	}
}

func TestSlackSearchSubcommands_PreferUserTokenOverBotToken(t *testing.T) {
	var seenAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.URL.Path+"="+r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"query":"roadmap","files":{"total":0,"matches":[]},"messages":{"total":0,"matches":[]}}`))
	}))
	defer srv.Close()
	setupSlackShadowTestEnv(t, srv.URL)
	// Set BOTH bot and user tokens; search.files and search.all must send xoxp-.
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-bot-should-not-be-used")
	t.Setenv("SLACK_USER_TOKEN", "xoxp-user-token-expected")

	for _, args := range [][]string{
		{"search-api", "files", "--query", "roadmap", "--json", "--no-learn", "--no-cache"},
		{"search-api", "all", "--query", "roadmap", "--json", "--no-learn", "--no-cache"},
	} {
		var stdout, stderr bytes.Buffer
		cmd := newRootCmd(&rootFlags{})
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v failed: %v (stderr=%s)", args, err, stderr.String())
		}
	}

	want := []string{
		"/search.files=Bearer xoxp-user-token-expected",
		"/search.all=Bearer xoxp-user-token-expected",
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

func TestSlackWebClient_BodyErrorExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode int
	}{
		{name: "invalid_auth", body: `{"ok":false,"error":"invalid_auth"}`, wantCode: 4},
		{name: "not_authed", body: `{"ok":false,"error":"not_authed"}`, wantCode: 4},
		{name: "token_expired", body: `{"ok":false,"error":"token_expired"}`, wantCode: 4},
		{name: "not_allowed_token_type", body: `{"ok":false,"error":"not_allowed_token_type"}`, wantCode: 4},
		{name: "ratelimited", body: `{"ok":false,"error":"ratelimited"}`, wantCode: 7},
		{name: "internal_error", body: `{"ok":false,"error":"internal_error"}`, wantCode: 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			setupSlackShadowTestEnv(t, srv.URL)
			t.Setenv("SLACK_SESSION_TOKEN", "xoxc-test-session")
			t.Setenv("SLACK_COOKIE_D", "xoxd-test-cookie")

			cmd := newRootCmd(&rootFlags{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"web-client", "unreads", "--json", "--no-learn"})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for %s", tc.body)
			}
			if got := ExitCode(err); got != tc.wantCode {
				t.Fatalf("ExitCode(%v) = %d, want %d", err, got, tc.wantCode)
			}
		})
	}
}
