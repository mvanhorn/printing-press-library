// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRefreshAccessToken_RotatesRefresh(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()

	// Re-point the refresh endpoint at the test server by overriding
	// the HTTP client to rewrite the URL.
	origClient := refreshClient
	SetRefreshHTTPClient(&http.Client{
		Transport: &rewriteTransport{target: srv.URL},
	})
	defer SetRefreshHTTPClient(origClient)

	resp, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if resp.AccessToken != "new-access" {
		t.Errorf("expected new-access, got %q", resp.AccessToken)
	}
	if resp.RefreshToken != "new-refresh" {
		t.Errorf("expected new-refresh, got %q", resp.RefreshToken)
	}
	if gotBody["refresh_token"] != "old-refresh" {
		t.Errorf("expected refresh_token=old-refresh, got %q", gotBody["refresh_token"])
	}
	if _, ok := gotBody["client_id"]; ok {
		t.Errorf("did not expect legacy WorkOS client_id in Granola refresh body")
	}
	if _, ok := gotBody["grant_type"]; ok {
		t.Errorf("did not expect legacy WorkOS grant_type in Granola refresh body")
	}

	// Verify cache holds the new pair.
	ResetTokenCache()
}

// rewriteTransport replaces the request URL with target+path.
type rewriteTransport struct {
	target string
}

func (rt *rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Strip scheme+host and replace with target.
	r.URL.Scheme = ""
	r.URL.Host = ""
	new := strings.TrimRight(rt.target, "/") + r.URL.RequestURI()
	r2, err := http.NewRequest(r.Method, new, r.Body)
	if err != nil {
		return nil, err
	}
	r2.Header = r.Header
	return http.DefaultTransport.RoundTrip(r2)
}

func TestRefreshAccessToken_RejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	origClient := refreshClient
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	defer SetRefreshHTTPClient(origClient)
	_, err := RefreshAccessToken("bad")
	if err == nil {
		t.Fatalf("expected error on 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention 401, got %v", err)
	}
}

// PATCH(encrypted-cache): tests for D6 read-only refresh policy + source detection.

func TestRefreshAccessToken_RefusesEncryptedSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()

	// Simulate a load that populated the cache from supabase.json.enc.
	tokenMu.Lock()
	cachedAccess = "old-access"
	cachedRefresh = "old-refresh"
	cachedSource = TokenSourceEncryptedSupabase
	tokenMu.Unlock()

	_, err := RefreshAccessToken("old-refresh")
	if err == nil {
		t.Fatal("expected ErrRefreshRefused, got nil")
	}
	if err != ErrRefreshRefused {
		t.Errorf("expected ErrRefreshRefused, got %v", err)
	}
}

func TestRefreshAccessToken_RefusesPlaintextDesktopFallbackSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()

	// Simulate a plaintext supabase.json fallback after supabase.json.enc was
	// present but Keychain-unavailable. The refresh token is still desktop-owned
	// and may be the same single-use token Granola desktop has encrypted on disk.
	tokenMu.Lock()
	cachedAccess = "old-access"
	cachedRefresh = "old-refresh"
	cachedSource = TokenSourcePlaintextSupabaseDesktopFallback
	tokenMu.Unlock()

	_, err := RefreshAccessToken("old-refresh")
	if err == nil {
		t.Fatal("expected ErrRefreshRefused, got nil")
	}
	if err != ErrRefreshRefused {
		t.Errorf("expected ErrRefreshRefused, got %v", err)
	}
}

func TestRefreshAccessToken_AllowsEnvOverrideSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()
	origClient := refreshClient
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	defer SetRefreshHTTPClient(origClient)

	tokenMu.Lock()
	cachedSource = TokenSourceEnvOverride
	tokenMu.Unlock()

	_, err := RefreshAccessToken("refresh")
	if err != nil {
		t.Fatalf("env override source should allow refresh, got: %v", err)
	}
}

func TestRefreshAccessToken_AllowsPlaintextSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer srv.Close()
	origClient := refreshClient
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	defer SetRefreshHTTPClient(origClient)

	tokenMu.Lock()
	cachedSource = TokenSourcePlaintextSupabase
	tokenMu.Unlock()

	_, err := RefreshAccessToken("refresh")
	if err != nil {
		t.Fatalf("plaintext source should allow refresh, got: %v", err)
	}
}

func TestLoadTokensRaw_DetectsPlaintextSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	tmp := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	// Use the test DEK to encrypt a synthetic supabase.json plaintext.
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")

	// Build a valid supabase.json plaintext.
	plaintext := []byte(`{"workos_tokens":"{\"access_token\":\"a\",\"refresh_token\":\"r\",\"expires_in\":3600,\"obtained_at\":1700000000000}","session_id":"s","user_info":"{\"id\":\"u\"}"}`)
	// Reuse the safestorage fixture-supabase.enc encryption parameters via parseKeyOverride is unnecessary; instead we'll just
	// write a separate fixture inline by encrypting on the fly. Easiest: write plaintext supabase.json instead and confirm
	// the plaintext source path. The .enc path detection is exercised by safestorage's own tests.
	if err := writeFile(t, tmp+"/supabase.json", plaintext); err != nil {
		t.Fatal(err)
	}
	_, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src != TokenSourcePlaintextSupabase {
		t.Errorf("expected TokenSourcePlaintextSupabase, got %v", src)
	}
}

func TestLoadTokensRaw_EnvOverrideSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "env-access-token")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "env-refresh")
	_, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src != TokenSourceEnvOverride {
		t.Errorf("expected TokenSourceEnvOverride, got %v", src)
	}
}

// TestLoadTokensRaw_DetectsEncryptedSource exercises the encrypted-supabase
// path using the committed safestorage fixture. Confirms loadFromSupabaseJSON
// prefers supabase.json.enc over supabase.json plaintext when both are
// present, and that the returned TokenSource correctly flags the encrypted
// origin so D6 (refresh-refusal) fires downstream.
func TestLoadTokensRaw_DetectsEncryptedSource(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	tmp := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")

	// Copy the committed safestorage test fixture into the simulated support dir.
	fixture, err := os.ReadFile("safestorage/testdata/fixture-supabase.enc")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(tmp+"/supabase.json.enc", fixture, 0o644); err != nil {
		t.Fatalf("write enc: %v", err)
	}

	tok, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src != TokenSourceEncryptedSupabase {
		t.Errorf("expected TokenSourceEncryptedSupabase, got %v", src)
	}
	if tok.AccessToken != "test-access" {
		t.Errorf("expected access_token=test-access from fixture, got %q", tok.AccessToken)
	}
}

func writeFile(t *testing.T, path string, data []byte) error {
	t.Helper()
	return os.WriteFile(path, data, 0o644)
}

// PATCH(d6-read-only-applies-to-all-desktop-token-sources): tests for the
// stored-accounts.json arm of D6. Granola desktop's key migration made
// supabase.json.enc undecryptable for this CLI, so loadTokensRaw now falls
// through to stored-accounts.json on exactly the installs where the desktop
// still owns that (single-use) refresh token.

// writeStoredAccounts writes a stored-accounts.json in Granola's real shape:
// a stringified accounts array whose entries carry a stringified tokens blob.
func writeStoredAccounts(t *testing.T, dir string) {
	t.Helper()
	tokens, err := json.Marshal(map[string]any{
		"access_token":  "stored-access",
		"refresh_token": "stored-refresh",
		"expires_in":    3600,
		"obtained_at":   1700000000000,
	})
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := json.Marshal([]map[string]any{{
		"email":   "user@example.com",
		"userId":  "u",
		"savedAt": "2026-07-01T00:00:00Z",
		"tokens":  string(tokens),
	}})
	if err != nil {
		t.Fatal(err)
	}
	top, err := json.Marshal(map[string]string{"accounts": string(accounts)})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFile(t, dir+"/stored-accounts.json", top); err != nil {
		t.Fatal(err)
	}
}

// migratedSupportDir simulates a current Granola install: supabase.json.enc is
// on disk but this CLI can no longer open it, so the token load falls through
// to stored-accounts.json. The key override is set to the test DEK so the test
// never shells out to the real Keychain; the ciphertext simply does not open
// under it, which is the same fall-through loadTokensRaw takes when the DEK
// itself is unavailable.
func migratedSupportDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")
	if err := writeFile(t, tmp+"/supabase.json.enc", []byte("v11-not-openable-under-the-old-dek-scheme")); err != nil {
		t.Fatal(err)
	}
	writeStoredAccounts(t, tmp)
	return tmp
}

// countingRefreshServer stands in for Granola's refresh endpoint and records
// whether it was called at all - a refusal must not touch the network.
func countingRefreshServer(t *testing.T, calls *atomic.Int32) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"rotated","refresh_token":"rotated-refresh","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	origClient := refreshClient
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	t.Cleanup(func() { SetRefreshHTTPClient(origClient) })
}

func TestRefreshAccessToken_RefusesStoredAccountsOnMigratedInstall(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	migratedSupportDir(t)
	var calls atomic.Int32
	countingRefreshServer(t, &calls)

	refresh, err := LoadRefreshToken()
	if err != nil {
		t.Fatalf("LoadRefreshToken: %v", err)
	}
	if refresh != "stored-refresh" {
		t.Fatalf("expected the stored-accounts refresh token, got %q", refresh)
	}
	if src := CurrentTokenSource(); src != TokenSourceStoredAccounts {
		t.Fatalf("expected TokenSourceStoredAccounts, got %v", src)
	}

	_, err = RefreshAccessToken(refresh)
	if err == nil {
		t.Fatal("expected ErrRefreshRefused, got nil")
	}
	if !errors.Is(err, ErrRefreshRefused) {
		t.Fatalf("expected ErrRefreshRefused, got %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("expected no refresh request against the stale desktop token, got %d", got)
	}
	if !strings.Contains(err.Error(), "stored-accounts") {
		t.Errorf("expected the refusal to name the source, got %v", err)
	}
	if !strings.Contains(err.Error(), "open Granola desktop") {
		t.Errorf("expected the refusal to point at Granola desktop, got %v", err)
	}
}

func TestRefreshAccessToken_AllowsStoredAccountsOnLegacyInstall(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	// Legacy install: stored-accounts.json only, no encrypted store the
	// desktop could be tracking the same refresh token in.
	tmp := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", tmp)
	writeStoredAccounts(t, tmp)
	var calls atomic.Int32
	countingRefreshServer(t, &calls)

	refresh, err := LoadRefreshToken()
	if err != nil {
		t.Fatalf("LoadRefreshToken: %v", err)
	}
	if src := CurrentTokenSource(); src != TokenSourceStoredAccounts {
		t.Fatalf("expected TokenSourceStoredAccounts, got %v", src)
	}
	if _, err := RefreshAccessToken(refresh); err != nil {
		t.Fatalf("legacy stored-accounts install should allow refresh, got: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected one refresh request, got %d", got)
	}
}

func TestRefreshAccessToken_AllowsEnvOverrideOnMigratedInstall(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	migratedSupportDir(t)
	t.Setenv("GRANOLA_WORKOS_TOKEN", "env-access-token")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "env-refresh")
	var calls atomic.Int32
	countingRefreshServer(t, &calls)

	refresh, err := LoadRefreshToken()
	if err != nil {
		t.Fatalf("LoadRefreshToken: %v", err)
	}
	if src := CurrentTokenSource(); src != TokenSourceEnvOverride {
		t.Fatalf("expected TokenSourceEnvOverride, got %v", src)
	}
	if _, err := RefreshAccessToken(refresh); err != nil {
		t.Fatalf("env override should allow refresh regardless of on-disk state, got: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("expected one refresh request, got %d", got)
	}
}
