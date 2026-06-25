// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
)

func loadCfg(t *testing.T, body string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return cfg
}

func TestEnsureFreshSession_PullsWhenHeaderEmpty(t *testing.T) {
	// No __client cookie => EnsureFreshJWT is a no-op; AuthSource=config (not env).
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\njwt='not.a.jwt'\n")
	called := 0
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { called++; return "h=1" }
	defer func() { readStudioCookieHeader = orig }()

	if err := EnsureFreshSession(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureFreshSession: %v", err)
	}
	if called != 1 {
		t.Errorf("cookie reader called %d times, want 1", called)
	}
	if cfg.StudioCookieHeader() != "h=1" {
		t.Errorf("StudioCookieHeader = %q, want h=1", cfg.StudioCookieHeader())
	}
}

func TestEnsureFreshSession_SkipsWhenHeaderPresent(t *testing.T) {
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\njwt='not.a.jwt'\nstudio_cookie_header='existing=1'\njwt_expiry=999\n")
	called := 0
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { called++; return "h=2" }
	defer func() { readStudioCookieHeader = orig }()

	if err := EnsureFreshSession(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureFreshSession: %v", err)
	}
	if called != 0 {
		t.Errorf("cookie reader called %d times, want 0 (header already present)", called)
	}
	if cfg.StudioCookieHeader() != "existing=1" {
		t.Errorf("StudioCookieHeader = %q, want existing=1 (unchanged)", cfg.StudioCookieHeader())
	}
}

func TestEnsureFreshSession_BrowserEmptyPersistsNothing(t *testing.T) {
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\njwt='not.a.jwt'\n")
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { return "" }
	defer func() { readStudioCookieHeader = orig }()

	if err := EnsureFreshSession(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureFreshSession: %v", err)
	}
	if cfg.StudioCookieHeader() != "" {
		t.Errorf("StudioCookieHeader = %q, want empty (nothing to persist)", cfg.StudioCookieHeader())
	}
	if cfg.SunoJwtExpiry != 0 {
		t.Errorf("SunoJwtExpiry = %d, want 0 (persist nothing when browser empty)", cfg.SunoJwtExpiry)
	}
}

func TestEnsureFreshSession_EnvAuthIsNoop(t *testing.T) {
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\n")
	cfg.AuthSource = "env:SUNO_JWT"
	called := 0
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { called++; return "h=3" }
	defer func() { readStudioCookieHeader = orig }()

	if err := EnsureFreshSession(context.Background(), cfg); err != nil {
		t.Fatalf("EnsureFreshSession: %v", err)
	}
	if called != 0 {
		t.Errorf("cookie reader called %d times, want 0 (env auth unmanaged)", called)
	}
	if cfg.StudioCookieHeader() != "" {
		t.Errorf("StudioCookieHeader = %q, want empty (env auth not persisted)", cfg.StudioCookieHeader())
	}
}

func TestRefreshStudioCookies_ForcesPullAndPersists(t *testing.T) {
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\njwt='not.a.jwt'\nstudio_cookie_header='old=1'\n")
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { return "fresh=2" }
	defer func() { readStudioCookieHeader = orig }()

	got := RefreshStudioCookies(context.Background(), cfg)
	if got != "fresh=2" {
		t.Errorf("RefreshStudioCookies = %q, want fresh=2", got)
	}
	if cfg.StudioCookieHeader() != "fresh=2" {
		t.Errorf("persisted header = %q, want fresh=2", cfg.StudioCookieHeader())
	}
}

func TestRefreshStudioCookies_EmptyKeepsOld(t *testing.T) {
	cfg := loadCfg(t, "base_url='https://studio-api-prod.suno.com'\njwt='not.a.jwt'\nstudio_cookie_header='old=1'\n")
	orig := readStudioCookieHeader
	readStudioCookieHeader = func(ctx context.Context) string { return "" }
	defer func() { readStudioCookieHeader = orig }()

	if got := RefreshStudioCookies(context.Background(), cfg); got != "old=1" {
		t.Errorf("RefreshStudioCookies = %q, want old=1 (keep stored when browser empty)", got)
	}
}
