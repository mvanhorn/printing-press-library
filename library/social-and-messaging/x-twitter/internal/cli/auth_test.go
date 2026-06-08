// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/config"
)

func TestAuthImportOAuth2StoresUserContextMetadata(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("oauth2_user_token = \"old-token\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--refresh-token", "refresh-token",
		"--scopes", "tweet.read,tweet.write,users.read,bookmark.read,dm.read,dm.write,follows.read,like.read,offline.access",
		"--expires-at", "2026-06-08T12:00:00Z",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["auth_lane"] != "oauth2_user_context" || payload["refresh_token_present"] != true {
		t.Fatalf("payload = %#v", payload)
	}
	if _, ok := payload["env_override_warning"]; ok {
		t.Fatalf("unexpected env_override_warning: %#v", payload)
	}
	if _, ok := payload["missing_for"]; ok {
		t.Fatalf("unexpected missing_for with complete imported scopes: %#v", payload)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.AccessToken != "user-token" || cfg.RefreshToken != "refresh-token" {
		t.Fatalf("tokens not stored in user-context fields: %+v", cfg)
	}
	if cfg.XOauth2UserToken != "" || cfg.UserContextAuthHeader() != "Bearer user-token" {
		t.Fatalf("imported token is shadowed: oauth2_user_token=%q header=%q", cfg.XOauth2UserToken, cfg.UserContextAuthHeader())
	}
	if len(cfg.Scopes) != 9 || cfg.Scopes[0] != "tweet.read" {
		t.Fatalf("scopes = %#v", cfg.Scopes)
	}
	if cfg.TokenExpiry.UTC().Format("2006-01-02T15:04:05Z") != "2026-06-08T12:00:00Z" {
		t.Fatalf("expiry = %s", cfg.TokenExpiry)
	}
}

func TestAuthImportOAuth2WarnsWhenEnvTokenWouldShadowImport(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "env-token")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "imported-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	warning, ok := payload["env_override_warning"].(string)
	if !ok || warning == "" {
		t.Fatalf("missing env_override_warning: %#v", payload)
	}
	if !strings.Contains(warning, "X_OAUTH2_USER_TOKEN") || !strings.Contains(warning, "shadow") {
		t.Fatalf("warning does not explain shadowing: %q", warning)
	}
}

func TestAuthImportOAuth2ReportsMissingScopeWorkflowsOnlyWhenPresent(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--scopes", "tweet.read",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	missing, ok := payload["missing_for"].(map[string]any)
	if !ok {
		t.Fatalf("missing_for should be present for incomplete scopes: %#v", payload)
	}
	if _, ok := missing["public_writes"]; !ok {
		t.Fatalf("missing_for should include public_writes: %#v", missing)
	}
}
