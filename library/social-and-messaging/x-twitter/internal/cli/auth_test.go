// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/config"
)

func TestAuthImportOAuth2StoresUserContextMetadata(t *testing.T) {
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
		"--refresh-token", "refresh-token",
		"--scopes", "tweet.read,tweet.write,users.read,offline.access",
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

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.AccessToken != "user-token" || cfg.RefreshToken != "refresh-token" {
		t.Fatalf("tokens not stored in user-context fields: %+v", cfg)
	}
	if len(cfg.Scopes) != 4 || cfg.Scopes[0] != "tweet.read" {
		t.Fatalf("scopes = %#v", cfg.Scopes)
	}
	if cfg.TokenExpiry.UTC().Format("2006-01-02T15:04:05Z") != "2026-06-08T12:00:00Z" {
		t.Fatalf("expiry = %s", cfg.TokenExpiry)
	}
}
