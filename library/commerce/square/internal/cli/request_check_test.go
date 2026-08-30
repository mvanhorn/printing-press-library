// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNovelRequestCheckHelpWires smoke-tests that the request check command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRequestCheckHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"request", "check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("request check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "check", "--approve-mutation"} {
		if !strings.Contains(help, want) {
			t.Fatalf("request check --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRequestCheckMatchesGeneratedOperationAndRequiresMutationApproval(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "payment.json")
	if err := os.WriteFile(bodyPath, []byte(`{"idempotency_key":"test-key","source_id":"test-source","amount_money":{"amount":100,"currency":"USD"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configData := "base_url = \"https://connect.squareup.com\"\n[headers]\nSquare-Version = \"2026-07-15\"\n"
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(approved bool) map[string]any {
		t.Helper()
		cmd := newNovelRequestCheckCmd(&rootFlags{configPath: configPath})
		args := []string{"--method", "POST", "--path", "/v2/payments", "--body", bodyPath}
		if approved {
			args = append(args, "--approve-mutation")
		}
		cmd.SetArgs(args)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v\n%s", err, out.String())
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("decode: %v\n%s", err, out.String())
		}
		return result
	}

	unapproved := run(false)
	if unapproved["valid"] != false || unapproved["safe_to_send"] != false {
		t.Fatalf("unapproved result = %#v", unapproved)
	}
	if unapproved["matched_operation_path"] != "/v2/payments" {
		t.Fatalf("matched path = %v", unapproved["matched_operation_path"])
	}

	approved := run(true)
	if approved["valid"] != true || approved["safe_to_send"] != false || approved["mutation_approved"] != true || approved["ready_for_manual_review"] != true {
		t.Fatalf("approved result = %#v", approved)
	}
	schema, ok := approved["schema_validation"].(map[string]any)
	if !ok || schema["available"] != false {
		t.Fatalf("schema validation disclosure = %#v", approved["schema_validation"])
	}
}

func TestClassifySquareBaseURLUsesExactHostname(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"https://connect.squareup.com", "production"},
		{"https://connect.squareup.com/", "production"},
		{"https://connect.squareup.com:443", "production"},
		{"https://connect.squareupsandbox.com", "sandbox"},
		{"http://connect.squareup.com", "custom/unknown"},
		{"https://user:secret@connect.squareup.com", "custom/unknown"},
		{"https://connect.squareup.com:8443", "custom/unknown"},
		{"https://connect.squareup.com/v2", "custom/unknown"},
		{"https://connect.squareup.com?token=secret", "custom/unknown"},
		{"https://connect.squareup.com#secret", "custom/unknown"},
		{"https://connect.squareup.com.evil.example", "custom/unknown"},
		{"https://squareupsandbox.com.evil.example", "custom/unknown"},
		{"https://example.test/square", "custom/unknown"},
	}
	for _, tt := range tests {
		got, err := classifySquareBaseURL(tt.raw)
		if err != nil || got != tt.want {
			t.Errorf("classifySquareBaseURL(%q) = %q, %v; want %q", tt.raw, got, err, tt.want)
		}
	}
	for _, raw := range []string{"not a URL", "connect.squareup.com", "file:///tmp/config"} {
		if got, err := classifySquareBaseURL(raw); err == nil {
			t.Errorf("classifySquareBaseURL(%q) = %q, nil; want error", raw, got)
		}
	}
}

func TestRequestCheckSanitizesCredentialBearingBaseURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configData := "base_url = \"https://alice:supersecret@connect.squareup.com/v2?token=hidden#fragment\"\n[headers]\nSquare-Version = \"2026-07-15\"\n"
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelRequestCheckCmd(&rootFlags{configPath: configPath})
	cmd.SetArgs([]string{"--method", "GET", "--path", "/v2/locations"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "supersecret") || strings.Contains(out.String(), "hidden") || strings.Contains(out.String(), "fragment") {
		t.Fatalf("credential-bearing URL components leaked in output: %s", out.String())
	}
	var result struct {
		Environment string `json:"environment"`
		BaseURL     string `json:"base_url"`
		Ready       bool   `json:"ready_for_manual_review"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Environment != "custom/unknown" || result.BaseURL != "https://connect.squareup.com" || result.Ready {
		t.Fatalf("unexpected sanitized URL result: %+v", result)
	}
}

func TestRequestCheckDryRunStillValidates(t *testing.T) {
	cmd := newNovelRequestCheckCmd(&rootFlags{dryRun: true})
	cmd.SetArgs([]string{"--method", "BANANA", "--path", "/v2/payments"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid value") {
		t.Fatalf("invalid dry-run error = %v", err)
	}
}

func TestRequestCheckReportsUnsetAPIVersion(t *testing.T) {
	cmd := newNovelRequestCheckCmd(&rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")})
	cmd.SetArgs([]string{"--method", "GET", "--path", "/v2/locations"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		APIVersion    string `json:"api_version"`
		VersionSource string `json:"api_version_source"`
		Ready         bool   `json:"ready_for_manual_review"`
		Checks        []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.APIVersion != "" || result.VersionSource != "not configured" || result.Ready {
		t.Fatalf("unexpected version result: %+v", result)
	}
	foundWarning := false
	for _, check := range result.Checks {
		if check.Name == "api_version" && check.Status == "warning" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("missing API version warning: %+v", result.Checks)
	}
}

func TestReadRequestBodyObjectEnforcesLimit(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "too-large.json")
	if err := os.WriteFile(bodyPath, bytes.Repeat([]byte(" "), int(maxRequestCheckBodyBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRequestBodyObject(bodyPath); err == nil || !strings.Contains(err.Error(), "exceeds 10 MiB") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequestCheckRejectsUnknownOperationWithoutNetwork(t *testing.T) {
	cmd := newNovelRequestCheckCmd(&rootFlags{configPath: filepath.Join(t.TempDir(), "missing.toml")})
	cmd.SetArgs([]string{"--method", "POST", "--path", "/v2/definitely-not-a-square-operation", "--approve-mutation"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Valid bool `json:"valid"`
		Safe  bool `json:"safe_to_send"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Valid || result.Safe {
		t.Fatalf("unknown operation unexpectedly passed: %+v", result)
	}
}

func TestMatchOperationPathSupportsConcreteResourceIDs(t *testing.T) {
	operation, ok := findRequestOperation("GET", "/v2/payments/PAYMENT_123")
	if !ok || operation.Path != "/v2/payments/{payment_id}" {
		t.Fatalf("operation = %+v, found = %v", operation, ok)
	}
}

func TestRequestCheckRejectsNullBody(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "null.json")
	if err := os.WriteFile(bodyPath, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelRequestCheckCmd(&rootFlags{})
	cmd.SetArgs([]string{"--method", "POST", "--path", "/v2/payments", "--body", bodyPath})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "JSON object, not null") {
		t.Fatalf("error = %v", err)
	}
}
