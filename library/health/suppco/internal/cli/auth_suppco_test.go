package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/config"
)

func TestAuthSetTokenReadsFromStdinAndDoesNotEcho(t *testing.T) {
	t.Setenv("SUPPCO_HOME", t.TempDir())
	t.Setenv("SUPPCO_ACCESS_TOKEN", "")

	flags := &rootFlags{}
	cmd := newAuthSetTokenCmd(flags)
	cmd.SetIn(strings.NewReader("fixture-token-value\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-token: %v", err)
	}
	if strings.Contains(out.String(), "fixture-token-value") {
		t.Fatalf("command output echoed the token: %q", out.String())
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if cfg.AccessToken != "fixture-token-value" {
		t.Fatalf("saved token = %q, want synthetic value", cfg.AccessToken)
	}
}

func TestAuthSetTokenRejectsPositionalToken(t *testing.T) {
	t.Setenv("SUPPCO_HOME", t.TempDir())
	t.Setenv("SUPPCO_ACCESS_TOKEN", "")

	flags := &rootFlags{}
	cmd := newAuthSetTokenCmd(flags)
	cmd.SetArgs([]string{"fixture-token-value"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
		t.Fatalf("positional token error = %v, want a no-arguments usage error", err)
	}
}

func TestAuthSetTokenWarnsWhenEnvironmentOverridesSavedToken(t *testing.T) {
	t.Setenv("SUPPCO_HOME", t.TempDir())
	t.Setenv("SUPPCO_ACCESS_TOKEN", "synthetic-env-secret")

	flags := &rootFlags{asJSON: true}
	cmd := newAuthSetTokenCmd(flags)
	cmd.SetIn(strings.NewReader("synthetic-saved-secret\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-token: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v (output %q)", err, out.String())
	}
	if got["note"] != "SUPPCO_ACCESS_TOKEN env var is still set and overrides the saved token" {
		t.Fatalf("note = %#v", got["note"])
	}
	if strings.Contains(out.String(), "synthetic-env-secret") || strings.Contains(out.String(), "synthetic-saved-secret") {
		t.Fatalf("command output leaked a token: %q", out.String())
	}
}

func TestAuthSetTokenRejectsEmptyStdin(t *testing.T) {
	t.Setenv("SUPPCO_HOME", t.TempDir())
	t.Setenv("SUPPCO_ACCESS_TOKEN", "")

	cmd := newAuthSetTokenCmd(&rootFlags{})
	cmd.SetIn(strings.NewReader("\n"))
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "token is required on stdin") {
		t.Fatalf("empty stdin error = %v", err)
	}
}

func TestAuthSetTokenRemovesLegacyAuthorizationAndPreservesOtherHeaders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SUPPCO_HOME", home)
	t.Setenv("SUPPCO_ACCESS_TOKEN", "synthetic-environment-token")
	configDir := filepath.Join(home, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	contents := []byte("auth_header = 'Bearer stale-header'\n[headers]\nAuthorization = 'Bearer stale-generic'\nX-Synthetic = 'keep'\n")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SUPPCO_CONFIG", configPath)

	cmd := newAuthSetTokenCmd(&rootFlags{})
	cmd.SetIn(strings.NewReader("synthetic-replacement-token\n"))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SUPPCO_ACCESS_TOKEN", "")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.AuthHeader(); got != "Bearer synthetic-replacement-token" {
		t.Fatalf("active authorization = %q", got)
	}
	for name := range cfg.Headers {
		if strings.EqualFold(name, "Authorization") {
			t.Fatalf("legacy Authorization header remained: %#v", cfg.Headers)
		}
	}
	if cfg.Headers["X-Synthetic"] != "keep" {
		t.Fatalf("unrelated headers = %#v", cfg.Headers)
	}
}
