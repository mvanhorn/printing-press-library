// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"
)

func testIDToken(expiry time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix())))
	return "header." + payload + ".signature"
}

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRefreshCardataAccessTokenPersistsRotatedCredential(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.AuthHeaderVal = "Bearer legacy"
	cfg.Headers = map[string]string{"X-Operator-Preference": "keep"}
	if err := cfg.SaveTokens("client-id", "client-secret", "expired-access", "old-refresh", now.Add(-time.Minute)); err != nil {
		t.Fatalf("save expired tokens: %v", err)
	}
	idToken := testIDToken(now.Add(30 * time.Minute))
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{
		AccessToken: "expired-access", RefreshToken: "old-refresh",
		IDToken: idToken, GCID: "stream-gcid",
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
	if reloaded.AuthHeaderVal != "" || reloaded.AuthHeader() != "Bearer new-access" || reloaded.Headers["X-Operator-Preference"] != "keep" {
		t.Fatal("OAuth refresh did not clear the shadowing auth header while preserving unrelated headers")
	}
	if !reloaded.TokenExpiry.Equal(now.Add(time.Hour)) {
		t.Fatalf("token expiry = %v, want %v", reloaded.TokenExpiry, now.Add(time.Hour))
	}
	session, err := loadCardataSession(reloaded)
	if err != nil {
		t.Fatalf("load refreshed session: %v", err)
	}
	if session["access_token"] != "new-access" || session["refresh_token"] != "old-refresh" ||
		session["id_token"] != idToken || session["gcid"] != "stream-gcid" {
		t.Fatalf("streaming session was not refreshed without losing identity fields: %#v", session)
	}
}

func TestSaveCardataOAuthTokensClearsLegacyHeaderForLogin(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AuthHeaderVal = "Bearer legacy"
	cfg.Headers = map[string]string{"X-Operator-Preference": "keep"}
	if err := saveCardataOAuthTokens(cfg, "client-id", "", &cardataToken{AccessToken: "new-access", RefreshToken: "refresh"}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.Load(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AuthHeaderVal != "" || reloaded.AuthHeader() != "Bearer new-access" || reloaded.Headers["X-Operator-Preference"] != "keep" {
		t.Fatal("login did not replace legacy auth header or preserved headers")
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

func TestRefreshCardataAccessTokenDropsExpiredStreamingToken(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client-id", "", "old-access", "old-refresh", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	if err := RefreshCardataAccessTokenIfNeeded(context.Background(), cfg, now, server.URL); err != nil {
		t.Fatal(err)
	}
	session, err := loadCardataSession(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if session["id_token"] != "" || session["access_token"] != "new-access" {
		t.Fatal("expired streaming ID token survived refresh")
	}
}

func TestCurrentCardataStreamSessionRenewsExpiredIDToken(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client-id", "", "valid-access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	newID := testIDToken(now.Add(2 * time.Hour))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh", "id_token": newID, "expires_in": 3600})
	}))
	defer server.Close()
	session, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || session["id_token"] != newID {
		t.Fatal("stream did not renew its expired ID token before use")
	}
}

func TestCurrentCardataStreamSessionRejectsMissingReplacementIDToken(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client-id", "", "access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	session, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL)
	if session != nil || !errors.Is(err, ErrCardataLoginRequired) {
		t.Fatal("stream accepted missing replacement ID token")
	}
}

func TestRefreshCardataAccessTokenSerializesStaleReaders(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	initial, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client-id", "", "old-access", "old-refresh", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	first, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("refresh_token") != "old-refresh" {
			t.Error("unexpected refresh token submitted")
		}
		time.Sleep(25 * time.Millisecond)
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, cfg := range []*config.Config{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- RefreshCardataAccessTokenIfNeeded(context.Background(), cfg, now, server.URL)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 || first.AccessToken != "new-access" || second.AccessToken != "new-access" {
		t.Fatal("stale readers rotated a refresh token more than once")
	}
}

func TestCurrentCardataStreamSessionRechecksIdentityUnderLock(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	initial, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client-id", "", "valid-access", "old-refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(initial, initial.ClientID, &cardataToken{IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, initial.TokenExpiry); err != nil {
		t.Fatal(err)
	}
	first, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	newID := testIDToken(now.Add(2 * time.Hour))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(25 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh", "id_token": newID, "expires_in": 3600})
	}))
	defer server.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, cfg := range []*config.Config{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL)
			if err == nil && session["id_token"] != newID {
				err = fmt.Errorf("streaming identity was not renewed")
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("streaming identity refreshed %d times, want once", calls.Load())
	}
}

func TestCardataRefreshTokenDistinguishesLoginFromTemporaryFailure(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantLogin bool
	}{
		{"rejected credential", http.StatusBadRequest, `{"error":"invalid_grant"}`, true},
		{"temporary provider failure", http.StatusServiceUnavailable, `{"error":"temporarily_unavailable"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := cardataRefreshToken(context.Background(), server.URL, "client", "", "refresh")
			if err == nil || errors.Is(err, ErrCardataLoginRequired) != tc.wantLogin || errors.Is(err, ErrCardataRefreshUnavailable) == tc.wantLogin {
				t.Fatalf("refresh classification = %v", err)
			}
		})
	}
}

func TestSetTokenClearsOldOAuthExpiryAndStreamingSession(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client-id", "", "old-access", "old-refresh", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{IDToken: testIDToken(time.Now().Add(time.Hour)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
		t.Fatal(err)
	}
	cmd := newAuthSetTokenCmd(&rootFlags{configPath: configPath})
	cmd.SetArgs([]string{"new-direct"})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccessToken != "new-direct" || !reloaded.TokenExpiry.IsZero() || reloaded.RefreshToken != "" || reloaded.ClientID != "" {
		t.Fatal("direct token retained stale OAuth refresh fields")
	}
	if _, err := os.Stat(cardataSessionPath(reloaded)); !os.IsNotExist(err) {
		t.Fatal("old streaming session remains after direct-token save")
	}
}

func TestRefreshDoesNotForwardCredentialBodyOnRedirect(t *testing.T) {
	var redirected atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", other.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer provider.Close()
	_, err := cardataRefreshToken(context.Background(), provider.URL, "client", "", "sensitive-refresh")
	if err == nil || redirected.Load() || strings.Contains(err.Error(), "sensitive-refresh") {
		t.Fatal("OAuth form body was forwarded or exposed after redirect")
	}
}

func TestStreamVerifyModeNeverRefreshesExpiredCredentials(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client-id", "", "expired-access", "refresh", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{IDToken: testIDToken(time.Now().Add(-time.Hour)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	previous := http.DefaultTransport
	http.DefaultTransport = testRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, fmt.Errorf("unexpected live OAuth request")
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	cmd := newStreamCmd(&rootFlags{configPath: configPath, timeout: time.Second})
	cmd.SetArgs([]string{"WBAJB3105JUV12345"})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil || calls.Load() != 0 {
		t.Fatalf("verify mode attempted credential refresh: error=%v calls=%d", err, calls.Load())
	}
}
