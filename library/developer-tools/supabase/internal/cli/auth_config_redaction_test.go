// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/client"
)

const syntheticCredentialSentinel = "synthetic-credential-must-not-escape"

func TestSanitizedAuthConfigRemainsSafeAcrossOutputAndDeliverySurfaces(t *testing.T) {
	raw := json.RawMessage(`{
		"site_url":"https://portal.example.test",
		"external_google_enabled":true,
		"external_google_secret":"synthetic-credential-must-not-escape-google",
		"smtp_pass":"synthetic-credential-must-not-escape-smtp",
		"future_unknown_credential":"synthetic-credential-must-not-escape-future"
	}`)
	sanitized, err := client.SanitizeAuthConfigResponse(raw)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		flags rootFlags
	}{
		{name: "json", flags: rootFlags{asJSON: true}},
		{name: "csv", flags: rootFlags{csv: true}},
		{name: "plain", flags: rootFlags{plain: true}},
		{name: "compact", flags: rootFlags{compact: true}},
		{name: "select cannot restore a secret", flags: rootFlags{asJSON: true, selectFields: "site_url,external_google_secret"}},
		{name: "quiet", flags: rootFlags{quiet: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := printOutputWithFlags(&output, sanitized, &tc.flags); err != nil {
				t.Fatalf("printOutputWithFlags() error = %v", err)
			}
			assertNoCredentialSentinel(t, output.Bytes())
		})
	}

	filePath := filepath.Join(t.TempDir(), "auth-config.json")
	if err := Deliver(DeliverSink{Scheme: "file", Target: filePath}, sanitized, false); err != nil {
		t.Fatalf("Deliver(file) error = %v", err)
	}
	fileBody, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	assertNoCredentialSentinel(t, fileBody)

	var webhookBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookBody, _ = os.ReadFile(filePath)
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		webhookBody = body.Bytes()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := Deliver(DeliverSink{Scheme: "webhook", Target: server.URL}, sanitized, false); err != nil {
		t.Fatalf("Deliver(webhook) error = %v", err)
	}
	assertNoCredentialSentinel(t, webhookBody)
}

func TestGetAuthServiceSanitizesBeforeOutputAndPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SUPABASE_ACCESS_TOKEN", "synthetic-management-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/project-ref/config/auth" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"site_url":"https://portal.example.test",
			"external_google_enabled":true,
			"external_google_secret":"synthetic-credential-must-not-escape-google",
			"smtp_pass":"synthetic-credential-must-not-escape-smtp"
		}`))
	}))
	defer server.Close()
	t.Setenv("SUPABASE_BASE_URL", server.URL)

	cmd := RootCmd()
	cmd.SetArgs([]string{"projects", "config", "get-auth-service", "project-ref", "--json", "--data-source", "auto"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get-auth-service error = %v; stderr = %s", err, stderr.String())
	}
	assertNoCredentialSentinel(t, stdout.Bytes())
	assertNoCredentialSentinel(t, stderr.Bytes())
	if !strings.Contains(stdout.String(), "https://portal.example.test") {
		t.Fatalf("approved field missing from output: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".cache", "supabase-pp-cli", "http")); !os.IsNotExist(err) {
		t.Fatalf("auth config response created an HTTP cache directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "supabase-pp-cli", "data.db")); !os.IsNotExist(err) {
		t.Fatalf("auth config response created a local-store database: %v", err)
	}
}

func TestUpdateAuthServiceRejectsSecretValuedFlagsWithoutEchoingValues(t *testing.T) {
	for _, flagName := range []string{"--external-google-secret", "--smtp-pass"} {
		t.Run(flagName, func(t *testing.T) {
			cmd := RootCmd()
			cmd.SetArgs([]string{
				"projects", "config", "update-auth-service", "project-ref",
				flagName, syntheticCredentialSentinel,
			})
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("secret-valued flag was accepted")
			}
			assertNoCredentialSentinel(t, stdout.Bytes())
			assertNoCredentialSentinel(t, stderr.Bytes())
			assertNoCredentialSentinel(t, []byte(err.Error()))
			if !strings.Contains(err.Error(), "--stdin") {
				t.Fatalf("rejection omitted stdin guidance: %v", err)
			}
		})
	}
}

func TestUpdateAuthServiceHidesSecretValuedFlagsAndMCPCommand(t *testing.T) {
	cmd := newProjectsConfigUpdateAuthServiceCmd(&rootFlags{})
	if cmd.Annotations["mcp:hidden"] != "true" {
		t.Fatal("credential-bearing auth-config update command is not hidden from MCP mirroring")
	}
	for _, name := range []string{"external-google-secret", "smtp-pass"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("expected rejected compatibility flag %q to remain parseable", name)
		}
		if !flag.Hidden {
			t.Errorf("secret-valued flag %q is still advertised in help", name)
		}
	}
}

func TestUpdateAuthServiceAcceptsCredentialFieldsThroughStdinAndRedactsResponse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SUPABASE_ACCESS_TOKEN", "synthetic-management-token")

	var requestBody bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		_, _ = requestBody.ReadFrom(r.Body)
		_, _ = w.Write([]byte(`{
			"site_url":"https://portal.example.test",
			"external_google_secret":"synthetic-credential-must-not-escape-response"
		}`))
	}))
	defer server.Close()
	t.Setenv("SUPABASE_BASE_URL", server.URL)

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stdinWriter.Write([]byte(`{"external_google_secret":"synthetic-credential-must-not-escape-request"}`)); err != nil {
		t.Fatal(err)
	}
	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = stdinReader
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = stdinReader.Close()
	})

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"projects", "config", "update-auth-service", "project-ref",
		"--stdin", "--json",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update-auth-service --stdin error = %v; stderr = %s", err, stderr.String())
	}
	if !bytes.Contains(requestBody.Bytes(), []byte(syntheticCredentialSentinel)) {
		t.Fatal("stdin credential was not sent to the protected endpoint")
	}
	assertNoCredentialSentinel(t, stdout.Bytes())
	assertNoCredentialSentinel(t, stderr.Bytes())
	if !strings.Contains(stdout.String(), "portal.example.test") {
		t.Fatalf("approved field missing from sanitized PATCH output: %s", stdout.String())
	}
}

func assertNoCredentialSentinel(t *testing.T, data []byte) {
	t.Helper()
	if bytes.Contains(data, []byte(syntheticCredentialSentinel)) {
		t.Fatalf("credential sentinel escaped: %s", data)
	}
}
