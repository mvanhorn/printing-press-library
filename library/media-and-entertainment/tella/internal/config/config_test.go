// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRetiredSessionCookieDoesNotAffectBearerAuth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TELLA_SESSION_COOKIE", "Cookie: session=retired-env-fixture")
	t.Setenv("TELLA_API_KEY", "bearer-fixture")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("session_cookie = \"retired-disk-fixture\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TellaApiKey != "bearer-fixture" || cfg.AuthSource != "env:TELLA_API_KEY" {
		t.Fatal("bearer environment authentication was not retained")
	}
	if err := cfg.SaveTokens("client-fixture", "client-secret-fixture", "access-fixture", "refresh-fixture", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "session_cookie") || strings.Contains(string(data), "retired-") {
		t.Fatal("retired cookie persisted")
	}
	t.Setenv("TELLA_API_KEY", "")
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "access-fixture" || loaded.RefreshToken != "refresh-fixture" || loaded.ClientID != "client-fixture" || loaded.ClientSecret != "client-secret-fixture" {
		t.Fatal("OAuth token persistence was not retained")
	}
	if loaded.envOverrides["SessionCookie"] {
		t.Fatal("retired cookie registered an environment override")
	}
}
