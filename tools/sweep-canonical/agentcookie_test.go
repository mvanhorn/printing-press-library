package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifestFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".printing-press.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// emitAgentcookieToml is a test helper that renders + writes in one step,
// mirroring what sweepCLI does in production. Tests can use this to
// verify end-to-end behavior without each test repeating the write boilerplate.
func emitAgentcookieToml(t *testing.T, dir string) (bool, error) {
	t.Helper()
	body, changed, err := agentcookieManifestForSweep(dir)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(filepath.Join(dir, agentcookieTomlFilename), []byte(body), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func TestSweepAgentcookieManifest_BearerToken(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "stripe",
  "cli_name": "stripe-pp-cli",
  "display_name": "Stripe",
  "description": "Payment processing and financial infrastructure API",
  "auth_type": "bearer_token",
  "auth_env_vars": ["STRIPE_SECRET_KEY"]
}`)
	changed, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("emitAgentcookieToml: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first emit")
	}
	body, _ := os.ReadFile(filepath.Join(dir, agentcookieTomlFilename))
	got := string(body)
	for _, want := range []string{
		"schema_version = 2",
		`name = "stripe-pp-cli"`,
		`display_name = "Stripe"`,
		`project_kind = "cli"`,
		"[secrets.file]",
		`path = "~/.config/stripe-pp-cli/config.toml"`,
		"[sync]",
		"default = false",
		"[sync.keys]",
		`"STRIPE_SECRET_KEY" = true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected emit to contain %q; got:\n%s", want, got)
		}
	}
}

func TestSweepAgentcookieManifest_SkipsCookieOnly(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "instacart",
  "cli_name": "instacart-pp-cli",
  "auth_type": "cookie"
}`)
	changed, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("emitAgentcookieToml: %v", err)
	}
	if changed {
		t.Error("expected changed=false for cookie-only CLI")
	}
	if _, err := os.Stat(filepath.Join(dir, agentcookieTomlFilename)); err == nil {
		t.Error("agentcookie.toml created for cookie-only CLI")
	}
}

func TestSweepAgentcookieManifest_Idempotent(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "stripe",
  "cli_name": "stripe-pp-cli",
  "display_name": "Stripe",
  "auth_type": "bearer_token",
  "auth_env_vars": ["STRIPE_SECRET_KEY"]
}`)
	changed1, err := emitAgentcookieToml(t, dir)
	if err != nil || !changed1 {
		t.Fatalf("first emit: changed=%v err=%v", changed1, err)
	}
	changed2, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("second emit: %v", err)
	}
	if changed2 {
		t.Error("expected idempotent second run (changed=false)")
	}
}

func TestSweepAgentcookieManifest_OverrideMarkerRespected(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "stripe",
  "cli_name": "stripe-pp-cli",
  "auth_type": "bearer_token",
  "auth_env_vars": ["STRIPE_SECRET_KEY"]
}`)
	override := "# agentcookie-manual-override\nname = \"hand-edited\"\n"
	if err := os.WriteFile(filepath.Join(dir, agentcookieTomlFilename), []byte(override), 0o644); err != nil {
		t.Fatalf("seed override: %v", err)
	}
	changed, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("emitAgentcookieToml: %v", err)
	}
	if changed {
		t.Error("expected changed=false when override marker present")
	}
	body, _ := os.ReadFile(filepath.Join(dir, agentcookieTomlFilename))
	if !strings.HasPrefix(string(body), agentcookieOverrideMarker) {
		t.Error("override file was overwritten")
	}
	if strings.Contains(string(body), "STRIPE_SECRET_KEY") {
		t.Error("override file gained generated content")
	}
}

func TestSweepAgentcookieManifest_MultipleEnvVarsSorted(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "example",
  "cli_name": "example-pp-cli",
  "auth_type": "oauth2",
  "auth_env_vars": ["EXAMPLE_CLIENT_SECRET", "EXAMPLE_CLIENT_ID"]
}`)
	_, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("emitAgentcookieToml: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, agentcookieTomlFilename))
	got := string(body)
	idIdx := strings.Index(got, "EXAMPLE_CLIENT_ID")
	secretIdx := strings.Index(got, "EXAMPLE_CLIENT_SECRET")
	if idIdx < 0 || secretIdx < 0 {
		t.Fatalf("expected both keys present; got:\n%s", got)
	}
	if idIdx > secretIdx {
		t.Errorf("expected EXAMPLE_CLIENT_ID before EXAMPLE_CLIENT_SECRET (alphabetical sort); got:\n%s", got)
	}
}

func TestSweepAgentcookieManifest_NoEnvVarsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "noauth-public-api",
  "cli_name": "noauth-public-api-pp-cli",
  "auth_type": "none"
}`)
	changed, err := emitAgentcookieToml(t, dir)
	if err != nil {
		t.Fatalf("emitAgentcookieToml: %v", err)
	}
	if changed {
		t.Error("expected changed=false for no-auth CLI")
	}
}

func TestAgentcookieManifestWouldChange(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `{
  "api_name": "stripe",
  "cli_name": "stripe-pp-cli",
  "auth_type": "bearer_token",
  "auth_env_vars": ["STRIPE_SECRET_KEY"]
}`)
	would, err := agentcookieManifestWouldChange(dir)
	if err != nil || !would {
		t.Fatalf("expected would-change=true on first probe; got would=%v err=%v", would, err)
	}
	if _, err := emitAgentcookieToml(t, dir); err != nil {
		t.Fatalf("emit: %v", err)
	}
	would, err = agentcookieManifestWouldChange(dir)
	if err != nil {
		t.Fatalf("second probe: %v", err)
	}
	if would {
		t.Error("expected would-change=false after emit (idempotent)")
	}
}
