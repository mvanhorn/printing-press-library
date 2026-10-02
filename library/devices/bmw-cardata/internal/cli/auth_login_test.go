// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"
)

func TestRefreshCardataAccessTokenPersistsRotatedCredential(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfg.SaveTokens("client-id", "client-secret", "expired-access", "old-refresh", now.Add(-time.Minute)); err != nil {
		t.Fatalf("save expired tokens: %v", err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{
		AccessToken: "expired-access", RefreshToken: "old-refresh",
		IDToken: "stream-id-token", GCID: "stream-gcid",
	}, cfg.TokenExpiry); err != nil {
		t.Fatalf("write initial session: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		want := map[string]string{
			"grant_type": "refresh_token", "client_id": "client-id",
			"client_secret": "client-secret", "refresh_token": "old-refresh",
		}
		for key, value := range want {
			if got := r.Form.Get(key); got != value {
				t.Errorf("form[%s] = %q, want %q", key, got, value)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access", "expires_in": 3600,
		})
	}))
	defer server.Close()

	if err := RefreshCardataAccessTokenIfNeeded(context.Background(), cfg, now, server.URL); err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.AccessToken != "new-access" || reloaded.RefreshToken != "old-refresh" {
		t.Fatalf("persisted tokens = access %q refresh %q", reloaded.AccessToken, reloaded.RefreshToken)
	}
	if !reloaded.TokenExpiry.Equal(now.Add(time.Hour)) {
		t.Fatalf("token expiry = %v, want %v", reloaded.TokenExpiry, now.Add(time.Hour))
	}
	session, err := loadCardataSession(reloaded)
	if err != nil {
		t.Fatalf("load refreshed session: %v", err)
	}
	if session["access_token"] != "new-access" || session["refresh_token"] != "old-refresh" ||
		session["id_token"] != "stream-id-token" || session["gcid"] != "stream-gcid" {
		t.Fatalf("streaming session was not refreshed without losing identity fields: %#v", session)
	}
}

func TestRefreshCardataAccessTokenLeavesDirectCredentialAlone(t *testing.T) {
	cfg := &config.Config{
		BmwCardataAccessToken: "direct-token",
		TokenExpiry:           time.Now().Add(-time.Hour),
	}
	if err := RefreshCardataAccessTokenIfNeeded(context.Background(), cfg, time.Now(), "http://invalid.example"); err != nil {
		t.Fatalf("direct credential should bypass OAuth refresh: %v", err)
	}
}

func TestRefreshCardataAccessTokenDoesNotEchoProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"access_token":"provider-secret-sentinel"}`))
	}))
	defer server.Close()

	_, err := cardataRefreshToken(context.Background(), server.URL, "client", "", "refresh")
	if err == nil || strings.Contains(err.Error(), "provider-secret-sentinel") {
		t.Fatalf("provider response was exposed in refresh error: %v", err)
	}
}
