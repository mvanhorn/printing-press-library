// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/config"
)

func TestCollectTokenRefreshStateReport(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")
	_, err := auth.NewStoreAt(tokenStorePath).Upsert("user@example.com", auth.AccountTokens{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expires:      time.Now().Add(-time.Minute).UnixMilli(),
		SuperhumanToken: auth.SuperhumanToken{
			Token:   "id",
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		},
		LastUsedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	got := collectTokenRefreshStateReport(cfg)
	if got["status"] != string(auth.RefreshStateExpiredAccessCanRefresh) {
		t.Fatalf("status = %#v, want %s", got["status"], auth.RefreshStateExpiredAccessCanRefresh)
	}
	if got["account_count"] != 1 {
		t.Fatalf("account_count = %#v, want 1", got["account_count"])
	}
	active, ok := got["active"].(auth.RefreshStateReport)
	if !ok {
		t.Fatalf("active = %T, want auth.RefreshStateReport", got["active"])
	}
	if active.AccessTokenState != "expired" {
		t.Fatalf("access token state = %s, want expired", active.AccessTokenState)
	}
}

func TestDoctorAutoRefreshActive(t *testing.T) {
	t.Setenv(autoRefreshEnvVar, "")
	if !doctorAutoRefreshActive(&rootFlags{}) {
		t.Fatal("expected auto refresh active by default")
	}
	if doctorAutoRefreshActive(&rootFlags{noRefresh: true}) {
		t.Fatal("expected --no-refresh to disable")
	}
	t.Setenv(autoRefreshEnvVar, "1")
	if doctorAutoRefreshActive(&rootFlags{}) {
		t.Fatal("expected env opt-out to disable")
	}
}

func TestBinaryAgeDaysAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	now := time.Unix(2000, 0)
	if err := os.Chtimes(path, now.Add(-49*time.Hour), now.Add(-49*time.Hour)); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	got, err := binaryAgeDaysAt(path, now)
	if err != nil {
		t.Fatalf("binaryAgeDaysAt: %v", err)
	}
	if got != 2 {
		t.Fatalf("age days = %d, want 2", got)
	}
}
