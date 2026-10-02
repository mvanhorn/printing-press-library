// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
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
	"github.com/spf13/cobra"
)

func testIDToken(expiry time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, expiry.Unix())))
	return "header." + payload + ".signature"
}

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNewClientPreviewAndLocalModesDoNotRefreshOAuth(t *testing.T) {
	for _, tc := range []struct {
		name, verify, liveHTTP, dataSource string
		dryRun, wantClientDryRun           bool
	}{
		{"dry-run", "", "", "", true, true},
		{"verify", "1", "", "", false, true},
		{"verify-mock-live", "1", "1", "", false, false},
		{"local", "", "", "local", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
			t.Setenv("PRINTING_PRESS_VERIFY", tc.verify)
			t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", tc.liveHTTP)
			cfg, err := config.Load(filepath.Join(home, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.SaveTokens("client", "", "expired-access", "single-use-refresh", time.Now().Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(cfg.Path)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			previous := http.DefaultTransport
			http.DefaultTransport = testRoundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return nil, fmt.Errorf("unexpected OAuth request")
			})
			t.Cleanup(func() { http.DefaultTransport = previous })
			flags := &rootFlags{configPath: cfg.Path, dryRun: tc.dryRun, dataSource: tc.dataSource, timeout: time.Second}
			client, err := flags.newClient()
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 || client.DryRun != tc.wantClientDryRun {
				t.Fatalf("preview/local mode attempted refresh or had wrong dry-run setting: calls=%d dry_run=%v", calls.Load(), client.DryRun)
			}
			after, err := os.ReadFile(cfg.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("preview/local mode changed saved OAuth credentials")
			}
		})
	}
}

func TestLocalDataSourceRefusesGeneratedMutationWithoutProviderCall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	t.Setenv("BMW_CARDATA_BASE_URL", server.URL)
	cfg, err := config.Load(filepath.Join(home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "access", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetArgs([]string{"--config", cfg.Path, "--data-source", "local", "customers", "create-container", "--name", "example"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--data-source local") {
		t.Fatalf("expected local-only refusal, got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("local-only mutation sent %d provider requests", calls.Load())
	}
}

func TestLocalDataSourceRefusesDirectStream(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	cmd := newStreamCmd(&rootFlags{dataSource: "local"})
	cmd.SetArgs([]string{"WBAJB3105JUV12345"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--data-source local") {
		t.Fatalf("expected local-only stream refusal, got %v", err)
	}
}

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
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken, IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, now.Add(-time.Hour)); err != nil {
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
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken, IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, now.Add(time.Hour)); err != nil {
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
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken, IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
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

func TestRefreshCardataAccessTokenSerializesConfigAliases(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	initial, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client-id", "", "old-access", "old-refresh", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	first, err := config.Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	second, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	firstPath, err := config.CanonicalPath(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	secondPath, err := config.CanonicalPath(second.Path)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath != secondPath {
		t.Fatal("aliases do not share a config target")
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
		t.Fatal("aliases rotated a single-use refresh token more than once")
	}
	if info, err := os.Lstat(alias); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("refresh replaced config alias: %v", err)
	}
}

func TestLogoutRemovesExistingSidecarBesideConfigAlias(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	initial, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client", "", "access", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.MkdirAll(aliasDir, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasDir, "config.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	selected, err := config.Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Path != alias {
		t.Fatalf("selected config path = %q, want alias", selected.Path)
	}
	legacySession, err := json.Marshal(map[string]string{
		"client_id": "client", "access_token": "access", "refresh_token": "refresh",
		"id_token": testIDToken(time.Now().Add(time.Hour)), "gcid": "gcid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(cardataSessionPath(selected), legacySession); err != nil {
		t.Fatal(err)
	}
	sharedPath, err := cardataSharedSessionPath(selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(sharedPath, legacySession); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCardataSession(selected); err != nil {
		t.Fatalf("shared session was not readable: %v", err)
	}
	cmd := newAuthLogoutCmd(&rootFlags{configPath: alias})
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cardataSessionPath(selected)); !os.IsNotExist(err) {
		t.Fatalf("old alias-side session remains after logout: %v", err)
	}
	if _, err := os.Stat(sharedPath); !os.IsNotExist(err) {
		t.Fatalf("shared session remains after logout: %v", err)
	}
	reloaded, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccessToken != "" || reloaded.RefreshToken != "" {
		t.Fatal("logout left OAuth credentials in symlink target")
	}
}

func TestLegacyAliasSessionMigratesForTargetStream(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	initial, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client", "", "access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.MkdirAll(aliasDir, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasDir, "config.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	selected, err := config.Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	idToken := testIDToken(now.Add(time.Hour))
	legacySession, err := json.Marshal(map[string]string{"client_id": "client", "access_token": "access", "refresh_token": "refresh", "id_token": idToken, "gcid": "gcid"})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(cardataSessionPath(selected), legacySession); err != nil {
		t.Fatal(err)
	}
	sharedPath, err := cardataSharedSessionPath(selected)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sharedPath); !os.IsNotExist(err) {
		t.Fatalf("shared sidecar unexpectedly exists before migration: %v", err)
	}
	fromAlias, err := currentCardataStreamSession(context.Background(), selected, now, "http://invalid.example")
	if err != nil || fromAlias["id_token"] != idToken {
		t.Fatalf("alias stream lost its valid session: %v", err)
	}
	if _, err := os.Stat(cardataSessionPath(selected)); !os.IsNotExist(err) {
		t.Fatalf("legacy sidecar remained after migration: %v", err)
	}
	info, err := os.Stat(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("shared sidecar permissions = %v", info.Mode().Perm())
	}
	fromTarget, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	targetSession, err := currentCardataStreamSession(context.Background(), fromTarget, now, "http://invalid.example")
	if err != nil || targetSession["id_token"] != idToken {
		t.Fatalf("target stream did not share migrated session: %v", err)
	}
}

func TestTargetFirstStreamMigratesKnownConfigAlias(t *testing.T) {
	for _, source := range []string{"default", "env"} {
		t.Run(source, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
			t.Setenv("BMW_CARDATA_CONFIG", "")
			now := time.Now().UTC()
			target := filepath.Join(home, "target", "config.toml")
			cfg, err := config.Load(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.SaveTokens("client", "", "access", "refresh", now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
			if source == "env" {
				alias = filepath.Join(home, "configured-alias", "config.toml")
				t.Setenv("BMW_CARDATA_CONFIG", alias)
			}
			if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			idToken := testIDToken(now.Add(time.Hour))
			legacyData, err := json.Marshal(map[string]string{"client_id": "client", "access_token": "access", "refresh_token": "refresh", "id_token": idToken, "gcid": "gcid"})
			if err != nil {
				t.Fatal(err)
			}
			legacy := filepath.Join(filepath.Dir(alias), "cardata_session.json")
			if err := config.WritePrivateFile(legacy, legacyData); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			// The target path is selected first. Migration must find only the
			// known matching alias, before an OAuth refresh can be attempted.
			session, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL)
			if err != nil || session["id_token"] != idToken || calls.Load() != 0 {
				t.Fatalf("target-first stream lost known alias session: err=%v refresh_calls=%d", err, calls.Load())
			}
			if _, err := os.Stat(legacy); !os.IsNotExist(err) {
				t.Fatalf("old alias session remained after migration: %v", err)
			}
			shared, err := cardataSharedSessionPath(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(shared); err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("shared session missing or not private: info=%v err=%v", info, err)
			}
		})
	}
}

func TestTargetFirstStreamRejectsConflictingKnownAliases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	target := filepath.Join(home, "target", "config.toml")
	cfg, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	envAlias := filepath.Join(home, "env-alias", "config.toml")
	t.Setenv("BMW_CARDATA_CONFIG", envAlias)
	defaultAlias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
	for i, alias := range []string{defaultAlias, envAlias} {
		if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, alias); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		data, err := json.Marshal(map[string]string{
			"client_id": "client", "access_token": "access", "refresh_token": "refresh",
			"id_token": testIDToken(now.Add(time.Duration(i+1) * time.Hour)), "gcid": "gcid",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := config.WritePrivateFile(filepath.Join(filepath.Dir(alias), "cardata_session.json"), data); err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	if _, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL); err == nil || !strings.Contains(err.Error(), "conflicting valid streaming sessions") {
		t.Fatalf("conflicting aliases did not fail closed: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("conflicting aliases triggered %d OAuth refresh calls", calls.Load())
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("conflicting aliases wrote shared session: %v", err)
	}
}

func TestLogoutThroughTargetRemovesKnownAliasSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	t.Setenv("BMW_CARDATA_CONFIG", "")
	target := filepath.Join(home, "target", "config.toml")
	cfg, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "access", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	legacy := filepath.Join(filepath.Dir(alias), "cardata_session.json")
	data, err := json.Marshal(map[string]string{"client_id": "client", "access_token": "access", "refresh_token": "refresh", "id_token": testIDToken(time.Now().Add(time.Hour)), "gcid": "gcid"})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(legacy, data); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(shared, data); err != nil {
		t.Fatal(err)
	}
	cmd := newAuthLogoutCmd(&rootFlags{configPath: target})
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("logout through target left known alias-side credentials: %v", err)
	}
}

func TestTargetAuthCleanupWithoutSharedSessionUsesSavedTokenProof(t *testing.T) {
	for _, tc := range []struct {
		name, action, aliasAccount string
		wantRemoved                bool
	}{
		{"logout-matching", "logout", "a", true},
		{"set-token-matching", "set-token", "a", true},
		{"logout-foreign", "logout", "b", false},
		{"set-token-foreign", "set-token", "b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
			t.Setenv("BMW_CARDATA_CONFIG", "")
			target := filepath.Join(home, "target", "config.toml")
			cfg, err := config.Load(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.SaveTokens("shared-client", "", "account-a-access", "account-a-refresh", time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
			if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			legacy := filepath.Join(filepath.Dir(alias), "cardata_session.json")
			data, err := json.Marshal(map[string]string{
				"client_id": "shared-client", "gcid": "account-" + tc.aliasAccount,
				"id_token":      testIDToken(time.Now().Add(time.Hour)),
				"access_token":  "account-" + tc.aliasAccount + "-access",
				"refresh_token": "account-" + tc.aliasAccount + "-refresh",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := config.WritePrivateFile(legacy, data); err != nil {
				t.Fatal(err)
			}
			shared, err := cardataSharedSessionPath(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(shared); !os.IsNotExist(err) {
				t.Fatalf("shared sidecar unexpectedly exists: %v", err)
			}
			var cmd *cobra.Command
			var args []string
			if tc.action == "logout" {
				cmd = newAuthLogoutCmd(&rootFlags{configPath: target})
			} else {
				cmd = newAuthSetTokenCmd(&rootFlags{configPath: target})
				args = []string{"direct-token"}
			}
			cmd.SetOut(io.Discard)
			cmd.SetContext(context.Background())
			if err := cmd.RunE(cmd, args); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(legacy)
			if tc.wantRemoved {
				if !os.IsNotExist(err) {
					t.Fatalf("matching alias credentials remained after %s: %v", tc.action, err)
				}
			} else if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("foreign account alias changed after %s: %v", tc.action, err)
			}
		})
	}
}

func TestSameClientDifferentAccountAliasIsNeitherMigratedNorDeleted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	t.Setenv("BMW_CARDATA_CONFIG", "")
	now := time.Now().UTC()
	target := filepath.Join(home, "target", "config.toml")
	cfg, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("shared-client", "", "account-a-access", "account-a-refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	legacy := filepath.Join(filepath.Dir(alias), "cardata_session.json")
	otherAccount, err := json.Marshal(map[string]string{
		"client_id": "shared-client", "gcid": "account-b", "id_token": testIDToken(now.Add(time.Hour)),
		"access_token": "account-b-access", "refresh_token": "account-b-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(legacy, otherAccount); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyCardataSession(context.Background(), cfg, now); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("another account's sidecar was migrated: %v", err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{
		AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken,
		IDToken: testIDToken(now.Add(time.Hour)), GCID: "account-a",
	}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(legacy); err != nil || !bytes.Equal(data, otherAccount) {
		t.Fatalf("new login changed another account's alias sidecar: %v", err)
	}
	cmd := newAuthLogoutCmd(&rootFlags{configPath: target})
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(legacy); err != nil || !bytes.Equal(data, otherAccount) {
		t.Fatalf("logout deleted another account's alias sidecar: %v", err)
	}
}

func TestForeignSharedSessionIsNotReplacedByMatchingAliasOrAPIRefresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	t.Setenv("BMW_CARDATA_CONFIG", "")
	now := time.Now().UTC()
	target := filepath.Join(home, "target", "config.toml")
	cfg, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("shared-client", "", "account-a-access", "account-a-refresh", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, ".config", "bmw-cardata-pp-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(alias), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	aliasData, err := json.Marshal(map[string]string{
		"client_id": "shared-client", "gcid": "account-a", "id_token": testIDToken(now.Add(time.Hour)),
		"access_token": "account-a-access", "refresh_token": "account-a-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(filepath.Dir(alias), "cardata_session.json")
	if err := config.WritePrivateFile(legacy, aliasData); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := json.Marshal(map[string]string{
		"client_id": "shared-client", "gcid": "account-b", "id_token": testIDToken(now.Add(time.Hour)),
		"access_token": "account-b-access", "refresh_token": "account-b-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(shared, foreign); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyCardataSession(context.Background(), cfg, now); err == nil {
		t.Fatal("matching alias replaced a current shared session from another account")
	}
	if data, err := os.ReadFile(shared); err != nil || !bytes.Equal(data, foreign) {
		t.Fatalf("migration changed another account's shared session: %v", err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"account-a-new-access","refresh_token":"account-a-new-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	if err := RefreshCardataAccessTokenIfNeeded(context.Background(), cfg, now, server.URL); err != nil {
		t.Fatalf("ordinary API refresh was blocked by foreign sidecar: %v", err)
	}
	if calls.Load() != 1 || cfg.AccessToken != "account-a-new-access" {
		t.Fatalf("API refresh did not rotate account A once: calls=%d", calls.Load())
	}
	if data, err := os.ReadFile(shared); err != nil || !bytes.Equal(data, foreign) {
		t.Fatalf("API refresh overwrote another account's shared session: %v", err)
	}
}

func TestSavedOAuthCleanupStillWorksWithDirectEnvironmentOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	configPath := filepath.Join(home, "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "saved-access", "saved-refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{
		AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken,
		IDToken: testIDToken(now.Add(time.Hour)), GCID: "saved-gcid",
	}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "different-account-direct-token")
	cmd := newAuthLogoutCmd(&rootFlags{configPath: configPath})
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("logout left the matching saved OAuth sidecar under env override: %v", err)
	}
	active, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if active.AuthHeader() != "Bearer different-account-direct-token" {
		t.Fatal("logout changed the direct environment credential")
	}
}

func TestLoginPersistenceWithEnvironmentOverrideDoesNotReportFalseFailure(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "direct-override")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tok := &cardataToken{AccessToken: "saved-access", RefreshToken: "saved-refresh", IDToken: testIDToken(now.Add(time.Hour)), GCID: "saved-gcid"}
	if err := saveCardataOAuthTokens(cfg, "client", "", tok, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, tok, now.Add(time.Hour)); err != nil {
		t.Fatalf("login persistence reported failure after saving matching tokens: %v", err)
	}
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadCardataSession(reloaded); err != nil {
		t.Fatalf("new saved OAuth session was not usable after removing env override: %v", err)
	}
}

func TestSharedSessionWithSameClientWrongTokensIsRejected(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("shared-client", "", "account-a-access", "account-a-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, access, refresh string }{
		{"different-account", "account-b-access", "account-b-refresh"},
		{"missing-token-proof", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(map[string]string{
				"client_id": "shared-client", "gcid": "account-b", "id_token": testIDToken(time.Now().Add(time.Hour)),
				"access_token": tc.access, "refresh_token": tc.refresh,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := config.WritePrivateFile(shared, data); err != nil {
				t.Fatal(err)
			}
			if _, err := loadCardataSession(cfg); err == nil {
				t.Fatal("reused an unproven shared session")
			}
		})
	}
}

func TestLogoutPreservesUnprovenSharedAccountSession(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("shared-client", "", "account-a-access", "account-a-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	otherAccount, err := json.Marshal(map[string]string{
		"client_id": "shared-client", "gcid": "account-b", "id_token": testIDToken(time.Now().Add(time.Hour)),
		"access_token": "account-b-access", "refresh_token": "account-b-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(shared, otherAccount); err != nil {
		t.Fatal(err)
	}
	cmd := newAuthLogoutCmd(&rootFlags{configPath: configPath})
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(shared); err != nil || !bytes.Equal(data, otherAccount) {
		t.Fatalf("logout deleted another account's shared session: %v", err)
	}
}

func TestDirectEnvironmentTokenCannotReuseSavedStreamingSession(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("client", "", "saved-access", "saved-refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{
		AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken,
		IDToken: testIDToken(now.Add(time.Hour)), GCID: "saved-gcid",
	}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "different-account-direct-token")
	withOverride, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := currentCardataStreamSession(context.Background(), withOverride, now, "http://invalid.example"); !errors.Is(err, ErrCardataLoginRequired) {
		t.Fatalf("environment token reused a different saved streaming identity: %v", err)
	}
}

func TestSidecarMigrationSelectsUsableSession(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	initial, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client", "", "access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.MkdirAll(aliasDir, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasDir, "config.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	selected, err := config.Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	sharedToken := testIDToken(now.Add(time.Hour))
	for _, item := range []struct{ path, idToken string }{
		{cardataSessionPath(selected), testIDToken(now.Add(-time.Hour))},
		{filepath.Join(filepath.Dir(target), "cardata_session.json"), sharedToken},
	} {
		data, err := json.Marshal(map[string]string{"client_id": "client", "access_token": "access", "refresh_token": "refresh", "id_token": item.idToken, "gcid": "gcid"})
		if err != nil {
			t.Fatal(err)
		}
		if err := config.WritePrivateFile(item.path, data); err != nil {
			t.Fatal(err)
		}
	}
	session, err := currentCardataStreamSession(context.Background(), selected, now, "http://invalid.example")
	if err != nil || session["id_token"] != sharedToken {
		t.Fatalf("stale alias replaced shared session: %v", err)
	}
	if _, err := os.Stat(cardataSessionPath(selected)); !os.IsNotExist(err) {
		t.Fatalf("stale alias sidecar remained: %v", err)
	}
	newAliasToken := testIDToken(now.Add(2 * time.Hour))
	for _, item := range []struct{ path, idToken string }{
		{cardataSessionPath(selected), newAliasToken},
		{filepath.Join(filepath.Dir(target), "cardata_session.json"), testIDToken(now.Add(-time.Hour))},
	} {
		data, err := json.Marshal(map[string]string{"client_id": "client", "access_token": "access", "refresh_token": "refresh", "id_token": item.idToken, "gcid": "gcid"})
		if err != nil {
			t.Fatal(err)
		}
		if err := config.WritePrivateFile(item.path, data); err != nil {
			t.Fatal(err)
		}
	}
	session, err = currentCardataStreamSession(context.Background(), selected, now, "http://invalid.example")
	if err != nil || session["id_token"] != newAliasToken {
		t.Fatalf("valid alias did not repair expired shared session: %v", err)
	}
	if _, err := os.Stat(cardataSessionPath(selected)); !os.IsNotExist(err) {
		t.Fatalf("promoted alias sidecar remained: %v", err)
	}
}

func TestUnusableAliasSidecarDoesNotBlockAPIRefresh(t *testing.T) {
	for _, tc := range []struct {
		name string
		data func(time.Time) []byte
	}{
		{"malformed", func(time.Time) []byte { return []byte("{invalid") }},
		{"foreign-client", func(now time.Time) []byte {
			data, _ := json.Marshal(map[string]string{"client_id": "other-client", "gcid": "gcid", "id_token": testIDToken(now.Add(time.Hour))})
			return data
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
			now := time.Now().UTC()
			dir := t.TempDir()
			target := filepath.Join(dir, "target", "config.toml")
			initial, err := config.Load(target)
			if err != nil {
				t.Fatal(err)
			}
			if err := initial.SaveTokens("client", "", "old-access", "old-refresh", now.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			aliasDir := filepath.Join(dir, "alias")
			if err := os.MkdirAll(aliasDir, 0o700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(aliasDir, "config.toml")
			if err := os.Symlink(target, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			selected, err := config.Load(alias)
			if err != nil {
				t.Fatal(err)
			}
			if err := config.WritePrivateFile(cardataSessionPath(selected), tc.data(now)); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
			}))
			defer server.Close()
			if session, err := currentCardataStreamSession(context.Background(), selected, now, server.URL); err == nil || session != nil {
				t.Fatal("stream accepted unusable alias-side session")
			}
			if calls.Load() != 0 {
				t.Fatal("stream called provider before rejecting unusable alias-side session")
			}
			if err := RefreshCardataAccessTokenIfNeeded(context.Background(), selected, now, server.URL); err != nil {
				t.Fatalf("API token refresh was blocked by optional sidecar: %v", err)
			}
			if calls.Load() != 1 {
				t.Fatalf("API refresh calls = %d, want 1", calls.Load())
			}
			reloaded, err := config.Load(target)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.AccessToken != "new-access" || reloaded.RefreshToken != "new-refresh" {
				t.Fatal("ordinary OAuth refresh did not persist new credentials")
			}
		})
	}
}

func TestForeignSharedSidecarCannotSupplyStreamingIdentity(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("current-client", "", "access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	foreignID := testIDToken(now.Add(2 * time.Hour))
	data, err := json.Marshal(map[string]string{"client_id": "other-client", "gcid": "gcid", "id_token": foreignID})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(shared, data); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCardataSession(cfg); err == nil {
		t.Fatal("foreign streaming sidecar was accepted")
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"access_token":"renewed-access","refresh_token":"renewed-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	session, err := currentCardataStreamSession(context.Background(), cfg, now, server.URL)
	if session != nil || !errors.Is(err, ErrCardataLoginRequired) {
		t.Fatalf("foreign ID token supplied stream: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("foreign shared session triggered %d OAuth refresh calls", calls.Load())
	}
}

func TestStreamingWithoutClientIDCannotUseOrMigrateSidecar(t *testing.T) {
	t.Setenv("BMW_CARDATA_ACCESS_TOKEN", "")
	now := time.Now().UTC()
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	initial, err := config.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("", "", "file-access", "refresh", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	aliasDir := filepath.Join(dir, "alias")
	if err := os.MkdirAll(aliasDir, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasDir, "config.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	selected, err := config.Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	foreignSession, err := json.Marshal(map[string]string{
		"client_id": "other-client", "gcid": "gcid", "id_token": testIDToken(now.Add(time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivateFile(cardataSessionPath(selected), foreignSession); err != nil {
		t.Fatal(err)
	}
	shared, err := cardataSharedSessionPath(selected)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	for _, phase := range []string{"legacy", "shared"} {
		if phase == "shared" {
			if _, err := os.Stat(shared); !os.IsNotExist(err) {
				t.Fatalf("legacy sidecar was migrated without a client ID: %v", err)
			}
			if err := config.WritePrivateFile(shared, foreignSession); err != nil {
				t.Fatal(err)
			}
		}
		session, err := currentCardataStreamSession(context.Background(), selected, now, server.URL)
		if session != nil || !errors.Is(err, ErrCardataLoginRequired) {
			t.Fatalf("%s sidecar supplied identity without client ID: %v", phase, err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("stream called provider %d times without client ID", calls.Load())
	}
	if err := RefreshCardataAccessTokenIfNeeded(context.Background(), selected, now, server.URL); err != nil {
		t.Fatalf("ordinary API access was blocked without client ID: %v", err)
	}
	if selected.AuthHeader() != "Bearer file-access" {
		t.Fatal("ordinary API credential became unavailable")
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
	if err := writeCardataSession(initial, initial.ClientID, &cardataToken{AccessToken: initial.AccessToken, RefreshToken: initial.RefreshToken, IDToken: testIDToken(now.Add(-time.Minute)), GCID: "gcid"}, initial.TokenExpiry); err != nil {
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
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken, IDToken: testIDToken(time.Now().Add(time.Hour)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
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
	if err := writeCardataSession(cfg, cfg.ClientID, &cardataToken{AccessToken: cfg.AccessToken, RefreshToken: cfg.RefreshToken, IDToken: testIDToken(time.Now().Add(-time.Hour)), GCID: "gcid"}, cfg.TokenExpiry); err != nil {
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
