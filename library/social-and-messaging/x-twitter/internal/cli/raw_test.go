// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawCommandRegistered(t *testing.T) {
	t.Parallel()

	cmd, _, err := RootCmd().Find([]string{"raw"})
	if err != nil {
		t.Fatalf("find raw: %v", err)
	}
	if cmd == nil || cmd.Name() != "raw" {
		t.Fatalf("raw command not registered: %#v", cmd)
	}
}

func TestParseRawHeaders(t *testing.T) {
	t.Parallel()

	got, err := parseRawHeaders([]string{"X-Test=one", "Accept: application/json"})
	if err != nil {
		t.Fatalf("parse headers: %v", err)
	}
	if got["X-Test"] != "one" || got["Accept"] != "application/json" {
		t.Fatalf("headers = %#v", got)
	}
	if _, err := parseRawHeaders([]string{"bad-header"}); err == nil {
		t.Fatal("expected malformed header error")
	}
	if _, err := parseRawHeaders([]string{"Authorization=Bearer custom"}); err == nil {
		t.Fatal("expected auth-sensitive header error")
	}
}

func TestReadRawBodyInlineAndFile(t *testing.T) {
	t.Parallel()

	inline, err := readRawBody(`{"text":"hello"}`, "")
	if err != nil {
		t.Fatalf("inline body: %v", err)
	}
	if inline.(map[string]any)["text"] != "hello" {
		t.Fatalf("inline body = %#v", inline)
	}

	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(`{"ids":["1","2"]}`), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	fromFile, err := readRawBody("", path)
	if err != nil {
		t.Fatalf("file body: %v", err)
	}
	ids := fromFile.(map[string]any)["ids"].([]any)
	if len(ids) != 2 || ids[0] != "1" {
		t.Fatalf("file body = %#v", fromFile)
	}
	if _, err := readRawBody(`{"a":1}`, path); err == nil {
		t.Fatal("expected mutually exclusive body error")
	}
}

func TestRawDryRunAgentOutput(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"raw", "GET", "/2/users/me",
		"--param", "user.fields=verified",
		"--dry-run",
		"--agent",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("raw dry-run failed: %v\noutput: %s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["dry_run"] != true || payload["sent"] != false {
		t.Fatalf("payload = %#v", payload)
	}
	req := payload["request"].(map[string]any)
	if req["method"] != "GET" || req["path"] != "/2/users/me" {
		t.Fatalf("request = %#v", req)
	}
	params := req["params"].(map[string]any)
	if params["user.fields"] != "verified" {
		t.Fatalf("params = %#v", params)
	}
	meta := payload["meta"].(map[string]any)
	if meta["auth_lane"] != "app_only_api" {
		t.Fatalf("meta = %#v", meta)
	}
	if strings.Contains(out.String(), "app-token") {
		t.Fatalf("raw output leaked token: %s", out.String())
	}
}

func TestRawRejectsRelativePathWithoutSlash(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"raw", "GET", "@attacker.test/2/users/me", "--dry-run", "--agent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid raw path error")
	}
	if !strings.Contains(err.Error(), "raw path must start with /") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawRejectsPlainHTTPAbsoluteURL(t *testing.T) {
	t.Parallel()

	if _, err := validateRawPath("http://api.x.com/2/users/me"); err == nil {
		t.Fatal("expected plaintext URL rejection")
	}
}

func TestRawRejectsDisallowedAbsoluteHost(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"raw", "GET", "https://evil.example/2/users/me", "--dry-run", "--agent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected disallowed host error")
	}
	if !strings.Contains(err.Error(), "absolute URL not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawDryRunCookieAuthLaneForAllowedXHost(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "app-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"raw", "GET", "https://x.com/i/api/graphql/test", "--dry-run", "--agent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("raw dry-run failed: %v\noutput: %s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	meta := payload["meta"].(map[string]any)
	if meta["auth_lane"] != "x_articles_cookie" {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestReadRawBodyFromStdin(t *testing.T) {
	old := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	os.Stdin = reader
	defer func() { os.Stdin = old }()

	if _, err := writer.WriteString(`{"text":"from stdin"}`); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	body, err := readRawBody("@-", "")
	if err != nil {
		t.Fatalf("stdin body: %v", err)
	}
	if body.(map[string]any)["text"] != "from stdin" {
		t.Fatalf("stdin body = %#v", body)
	}
}

func TestRawClassifiesAPIErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf("base_url = %q\nbearer_token = \"app-token\"\n", server.URL)), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "raw", "GET", "/fail", "--agent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected API error")
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d, want 4 (err=%v)", got, err)
	}
}
