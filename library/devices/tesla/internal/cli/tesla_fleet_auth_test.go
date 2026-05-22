// Tests for tesla auth fleet-* — CSRF state binding, port-8585 pre-flight,
// secret-file precedence, verify-mode short-circuit, and the fleet-status
// no-secret-leak contract. Live Tesla servers are never touched; tests use a
// local httptest.Server reachable via the TESLA_FLEET_AUTH_URL /
// TESLA_FLEET_API_URL env overrides.
package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// configFlags returns a *rootFlags that points config.Load at a fresh temp
// config.toml. Use this in every test so we never touch the real
// ~/.config/tesla-pp-cli/config.toml.
func configFlags(t *testing.T) *rootFlags {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	return &rootFlags{configPath: cfgPath}
}

// mintJWT builds a syntactically valid JWT with the given audience and scope.
// Signature segment is junk; we only decode the payload for fleet-status. Used
// to verify decodeJWTClaims and the fleet-status output shape.
func mintJWT(t *testing.T, aud, scope string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := map[string]any{
		"aud":   aud,
		"scope": scope,
		"iss":   "https://auth.tesla.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	cb, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(cb)
	sig := base64.RawURLEncoding.EncodeToString([]byte("not-a-real-signature"))
	return header + "." + payload + "." + sig
}

// TestFleetRegister_HappyPath drives fleet-register against a local stub that
// mimics the client_credentials grant and the partner_accounts endpoint.
// Asserts the [fleet] block is written with client_id and public_key_domain.
func TestFleetRegister_HappyPath(t *testing.T) {
	flags := configFlags(t)

	// Stub auth server: /oauth2/v3/token (client_credentials) returns a
	// short-lived partner token; /api/1/partner_accounts returns 200.
	authMux := http.NewServeMux()
	authMux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.PostForm.Get("grant_type") != "client_credentials" {
			http.Error(w, "wrong grant_type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": mintJWT(t, "https://fleet-api.prd.na.vn.cloud.tesla.com", "vehicle_cmds"),
			"expires_in":   28800,
			"token_type":   "Bearer",
		})
	})
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/1/partner_accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", 401)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"response":{"account_id":"stub-1"}}`))
	})
	authSrv := httptest.NewServer(authMux)
	defer authSrv.Close()
	apiSrv := httptest.NewServer(apiMux)
	defer apiSrv.Close()

	t.Setenv("TESLA_FLEET_AUTH_URL", authSrv.URL)
	t.Setenv("TESLA_FLEET_API_URL", apiSrv.URL)

	cmd := newFleetRegisterCmd(flags)
	cmd.SetArgs([]string{
		"--client-id", "client-abc",
		"--client-secret", "secret-xyz",
		"--public-key-domain", "keys.example.com",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-register: %v\noutput: %s", err, buf.String())
	}

	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	ft := cfg.FleetTokens()
	if ft.ClientID != "client-abc" {
		t.Errorf("Fleet.ClientID: got %q want client-abc", ft.ClientID)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("Fleet.PublicKeyDomain: got %q want keys.example.com", ft.PublicKeyDomain)
	}
	// Sanity: client_secret persisted (the user can re-mint a partner token
	// later without re-typing it).
	if ft.ClientSecret == "" {
		t.Errorf("Fleet.ClientSecret was not persisted")
	}
}

// TestFleetRegister_PartnerAccountsError surfaces the Tesla error body on
// 4xx and leaves the [fleet] block untouched.
func TestFleetRegister_PartnerAccountsError(t *testing.T) {
	flags := configFlags(t)

	authMux := http.NewServeMux()
	authMux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "partner-token-xyz",
			"expires_in":   28800,
		})
	})
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/1/partner_accounts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"invalid_domain","error_description":"key file not reachable"}`))
	})
	authSrv := httptest.NewServer(authMux)
	defer authSrv.Close()
	apiSrv := httptest.NewServer(apiMux)
	defer apiSrv.Close()

	t.Setenv("TESLA_FLEET_AUTH_URL", authSrv.URL)
	t.Setenv("TESLA_FLEET_API_URL", apiSrv.URL)

	cmd := newFleetRegisterCmd(flags)
	cmd.SetArgs([]string{
		"--client-id", "cid",
		"--client-secret", "csec",
		"--public-key-domain", "broken.example.com",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected fleet-register to fail on partner_accounts 400")
	}
	if !strings.Contains(err.Error(), "partner_accounts http 400") {
		t.Errorf("expected partner_accounts http 400 in error, got: %v", err)
	}
	// [fleet] block should not have been written.
	cfg, _ := config.Load(flags.configPath)
	if ft := cfg.FleetTokens(); ft.ClientID != "" || ft.PublicKeyDomain != "" {
		t.Errorf("expected no [fleet] block changes on failure; got %+v", ft)
	}
}

// TestFleetRegister_ClientCredentials401 surfaces invalid_credentials cleanly.
func TestFleetRegister_ClientCredentials401(t *testing.T) {
	flags := configFlags(t)

	authMux := http.NewServeMux()
	authMux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"invalid_credentials"}`))
	})
	authSrv := httptest.NewServer(authMux)
	defer authSrv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", authSrv.URL)

	cmd := newFleetRegisterCmd(flags)
	cmd.SetArgs([]string{
		"--client-id", "cid",
		"--client-secret", "wrong-secret",
		"--public-key-domain", "keys.example.com",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected fleet-register to fail on 401")
	}
	if !strings.Contains(err.Error(), "invalid_credentials") {
		t.Errorf("expected invalid_credentials in error, got: %v", err)
	}
}

// TestFleetRegister_SecretFileWinsOverFlag asserts --client-secret-file beats
// --client-secret when both are supplied.
func TestFleetRegister_SecretFileWinsOverFlag(t *testing.T) {
	flags := configFlags(t)

	// Build a secret file (mode 600) with a distinctive marker.
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "fleet-secret")
	if err := os.WriteFile(secretPath, []byte("from-file-marker\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	// Stub server records the secret it received.
	var receivedSecret string
	authMux := http.NewServeMux()
	authMux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		receivedSecret = r.PostForm.Get("client_secret")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "partner-token",
			"expires_in":   28800,
		})
	})
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/1/partner_accounts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	authSrv := httptest.NewServer(authMux)
	defer authSrv.Close()
	apiSrv := httptest.NewServer(apiMux)
	defer apiSrv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", authSrv.URL)
	t.Setenv("TESLA_FLEET_API_URL", apiSrv.URL)

	cmd := newFleetRegisterCmd(flags)
	cmd.SetArgs([]string{
		"--client-id", "cid",
		"--client-secret", "from-flag-marker",
		"--client-secret-file", secretPath,
		"--public-key-domain", "keys.example.com",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-register: %v\noutput: %s", err, buf.String())
	}
	if receivedSecret != "from-file-marker" {
		t.Errorf("expected secret-file value to win; received %q", receivedSecret)
	}
}

// TestFleetRegister_SecretFileMode rejects a world/group-readable file.
func TestFleetRegister_SecretFileMode(t *testing.T) {
	flags := configFlags(t)
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "loose-perms")
	if err := os.WriteFile(secretPath, []byte("oops"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	cmd := newFleetRegisterCmd(flags)
	cmd.SetArgs([]string{
		"--client-id", "cid",
		"--client-secret-file", secretPath,
		"--public-key-domain", "keys.example.com",
	})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error on world-readable client-secret-file")
	}
	if !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("expected chmod 600 hint in error, got: %v", err)
	}
}

// TestFleetLogin_PortInUseError asserts the clear-error contract when 8585 is
// already bound. We bind a probe listener on 8585 just for this test.
func TestFleetLogin_PortInUseError(t *testing.T) {
	// Skip if we can't bind 8585 for an unrelated reason — the contract test
	// requires us to own the port for the duration.
	probe, err := net.Listen("tcp", "127.0.0.1:8585")
	if err != nil {
		t.Skipf("can't bind 127.0.0.1:8585 in this env: %v", err)
	}
	defer probe.Close()

	flags := configFlags(t)
	// Seed a stored client_id so the command reaches the port-bind step.
	cfg, _ := config.Load(flags.configPath)
	if err := cfg.SaveFleetTokens("cid", "csec", "", "", time.Time{}, "keys.example.com", ""); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cmd := newFleetLoginCmd(flags)
	cmd.SetArgs([]string{"--no-open"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected port-in-use error; got success")
	}
	if !strings.Contains(err.Error(), "port 8585 in use") {
		t.Errorf("expected 'port 8585 in use' error wording, got: %v", err)
	}
}

// TestFleetLogin_HappyPath_CSRF drives the full OAuth flow: launch the
// callback server, simulate a browser redirect with the matching state, and
// assert tokens are persisted.
func TestFleetLogin_HappyPath_CSRF(t *testing.T) {
	if listenerBlocked() {
		t.Skip("can't bind 127.0.0.1:8585 in this env")
	}
	flags := configFlags(t)

	cfg, _ := config.Load(flags.configPath)
	if err := cfg.SaveFleetTokens("cid", "csec", "", "", time.Time{}, "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Build the stub auth server (token endpoint only — authorize is hit by
	// the user's browser in real life; here we synthesize the callback).
	tokenMux := http.NewServeMux()
	tokenMux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "authorization_code" {
			http.Error(w, "wrong grant_type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  mintJWT(t, "https://fleet-api.prd.na.vn.cloud.tesla.com", "openid offline_access vehicle_cmds"),
			"refresh_token": "fleet-refresh-1",
			"expires_in":    28800,
			"token_type":    "Bearer",
			"scope":         "openid offline_access vehicle_cmds",
		})
	})
	authSrv := httptest.NewServer(tokenMux)
	defer authSrv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", authSrv.URL)

	// Drive runFleetLoginFlow in a background goroutine, then hit /callback
	// ourselves to simulate the browser redirect. We have to extract the
	// state from the printed URL to send a matching one.
	type result struct {
		tok *fleetTokenResponse
		err error
	}
	resCh := make(chan result, 1)

	// We can't easily intercept the printed URL here, so directly call the
	// function and concurrently hit the callback with the state we read from
	// the captured stderr. To make this deterministic, fish the state out of
	// the URL by polling stderr.
	cobraCmd := &cobra.Command{}
	errBuf := &threadSafeBuffer{}
	cobraCmd.SetErr(errBuf)
	cobraCmd.SetOut(errBuf)

	go func() {
		tok, err := runFleetLoginFlow(cobraCmd, nil, "cid", authSrv.URL+"/oauth2/v3/authorize", authSrv.URL+"/oauth2/v3/token", fleetAPIAudience, false)
		resCh <- result{tok, err}
	}()

	state := waitForStateInBuf(t, errBuf, 5*time.Second)
	if state == "" {
		t.Fatalf("never saw state= in printed URL; stderr=%q", errBuf.String())
	}

	// Hit the callback with the matching state.
	cb := fmt.Sprintf("http://127.0.0.1:8585/callback?state=%s&code=auth-code-1", url.QueryEscape(state))
	resp, err := http.Get(cb)
	if err != nil {
		t.Fatalf("hit callback: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	r := <-resCh
	if r.err != nil {
		t.Fatalf("runFleetLoginFlow: %v\nstderr=%s", r.err, errBuf.String())
	}
	if r.tok.AccessToken == "" {
		t.Fatalf("empty access token")
	}
	if r.tok.RefreshToken != "fleet-refresh-1" {
		t.Errorf("RefreshToken: got %q", r.tok.RefreshToken)
	}
}

// TestFleetLogin_CSRFMismatch sends a mismatched state and asserts the flow
// rejects it without ever returning a code.
func TestFleetLogin_CSRFMismatch(t *testing.T) {
	if listenerBlocked() {
		t.Skip("can't bind 127.0.0.1:8585 in this env")
	}

	authSrv := httptest.NewServer(http.NewServeMux())
	defer authSrv.Close()

	cobraCmd := &cobra.Command{}
	errBuf := &threadSafeBuffer{}
	cobraCmd.SetErr(errBuf)
	cobraCmd.SetOut(errBuf)

	type result struct {
		tok *fleetTokenResponse
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		tok, err := runFleetLoginFlow(cobraCmd, nil, "cid", authSrv.URL+"/oauth2/v3/authorize", authSrv.URL+"/oauth2/v3/token", fleetAPIAudience, false)
		resCh <- result{tok, err}
	}()

	// Wait for the URL to be printed so we know the server is listening; we
	// don't actually need the state value here.
	if state := waitForStateInBuf(t, errBuf, 5*time.Second); state == "" {
		t.Fatalf("never saw state= in printed URL; stderr=%q", errBuf.String())
	}

	// Send a bogus state.
	cb := "http://127.0.0.1:8585/callback?state=wrong-state&code=ignored"
	resp, err := http.Get(cb)
	if err != nil {
		t.Fatalf("hit callback: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	r := <-resCh
	if r.err == nil {
		t.Fatalf("expected CSRF state mismatch error")
	}
	if !strings.Contains(r.err.Error(), "CSRF") && !strings.Contains(r.err.Error(), "state mismatch") {
		t.Errorf("expected CSRF/state-mismatch error, got: %v", r.err)
	}
}

// TestFleetRefresh_HappyPath stores a refresh token, drives fleet-refresh
// against a stub, asserts the access token is rotated.
func TestFleetRefresh_HappyPath(t *testing.T) {
	flags := configFlags(t)
	cfg, _ := config.Load(flags.configPath)
	if err := cfg.SaveFleetTokens("cid", "csec", "old-access", "old-refresh", time.Now().Add(-time.Hour), "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "refresh_token" {
			http.Error(w, "wrong grant_type", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-1",
			"refresh_token": "new-refresh-1",
			"expires_in":    28800,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", srv.URL)

	cmd := newFleetRefreshCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-refresh: %v\noutput: %s", err, buf.String())
	}

	cfg2, _ := config.Load(flags.configPath)
	ft := cfg2.FleetTokens()
	if ft.AccessToken != "new-access-1" {
		t.Errorf("AccessToken not rotated: got %q want new-access-1", ft.AccessToken)
	}
	if ft.RefreshToken != "new-refresh-1" {
		t.Errorf("RefreshToken not rotated: got %q want new-refresh-1", ft.RefreshToken)
	}
}

// TestFleetRefresh_NoStoredRefreshToken refuses to refresh when no refresh
// token is on file.
func TestFleetRefresh_NoStoredRefreshToken(t *testing.T) {
	flags := configFlags(t)
	cmd := newFleetRefreshCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when no refresh token stored")
	}
	if !strings.Contains(err.Error(), "no Fleet refresh token") {
		t.Errorf("expected 'no Fleet refresh token' error, got: %v", err)
	}
}

// TestFleetRefresh_RefreshTokenExpired surfaces a friendly hint to re-run
// fleet-login on a 401 invalid_grant.
func TestFleetRefresh_RefreshTokenExpired(t *testing.T) {
	flags := configFlags(t)
	cfg, _ := config.Load(flags.configPath)
	if err := cfg.SaveFleetTokens("cid", "csec", "", "stale-refresh", time.Now().Add(-time.Hour), "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", srv.URL)

	cmd := newFleetRefreshCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error on expired refresh token")
	}
	if !strings.Contains(err.Error(), "fleet-login") {
		t.Errorf("expected hint to run fleet-login, got: %v", err)
	}
}

// TestFleetStatus_NoSecretLeak is the canonical "fleet-status never prints
// secrets" regression test. Seeds the config with distinctive token literals
// then scans stdout+stderr for them.
func TestFleetStatus_NoSecretLeak(t *testing.T) {
	flags := configFlags(t)
	cfg, _ := config.Load(flags.configPath)

	const (
		secretAccess  = "ACCESS-TOKEN-LEAK-MARKER-7Q3"
		secretRefresh = "REFRESH-TOKEN-LEAK-MARKER-P9X"
		secretSecret  = "CLIENT-SECRET-LEAK-MARKER-V4Z"
	)
	// Build a JWT around the access marker so decodeJWTClaims has something to
	// chew on. The audience/scope are inert; the marker still appears in
	// raw form inside the JWT payload, so we test ALL three never reach stdout.
	jwt := mintJWT(t, "https://fleet-api.prd.na.vn.cloud.tesla.com", "vehicle_cmds")
	if err := cfg.SaveFleetTokens("client-id-public", secretSecret, secretAccess+"."+jwt, secretRefresh, time.Now().Add(time.Hour), "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := newFleetStatusCmd(flags)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-status: %v", err)
	}
	combined := stdout.String() + stderr.String()
	for _, marker := range []string{secretAccess, secretRefresh, secretSecret} {
		if strings.Contains(combined, marker) {
			t.Errorf("fleet-status leaked secret %q to output:\n%s", marker, combined)
		}
	}
	// Sanity: output should still report presence + audience.
	if !strings.Contains(combined, "access_token_present") {
		t.Errorf("expected access_token_present in output, got:\n%s", combined)
	}
}

// TestFleetStatus_AudienceAndScopes asserts the JWT-decoded audience and
// scopes come through cleanly.
func TestFleetStatus_AudienceAndScopes(t *testing.T) {
	flags := configFlags(t)
	cfg, _ := config.Load(flags.configPath)
	jwt := mintJWT(t, "https://fleet-api.prd.na.vn.cloud.tesla.com", "openid offline_access vehicle_cmds")
	if err := cfg.SaveFleetTokens("cid", "csec", jwt, "rtok", time.Now().Add(time.Hour), "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := newFleetStatusCmd(flags)
	flags.asJSON = true
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-status: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode status JSON: %v\noutput: %s", err, stdout.String())
	}
	aud, _ := got["audience"].([]any)
	foundAud := false
	for _, a := range aud {
		if s, _ := a.(string); strings.Contains(s, "fleet-api") {
			foundAud = true
		}
	}
	if !foundAud {
		t.Errorf("expected audience to include fleet-api host, got %v", aud)
	}
	if scopes, _ := got["scopes"].(string); !strings.Contains(scopes, "vehicle_cmds") {
		t.Errorf("expected scopes to include vehicle_cmds, got %q", scopes)
	}
}

// TestFleetVerifyMode_AllShortCircuit asserts every subcommand exits zero
// under PRINTING_PRESS_VERIFY=1 without hitting the network.
func TestFleetVerifyMode_AllShortCircuit(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	flags := configFlags(t)

	subs := map[string]*cobra.Command{
		"fleet-setup":    newFleetSetupCmd(flags),
		"fleet-register": newFleetRegisterCmd(flags),
		"fleet-login":    newFleetLoginCmd(flags),
		"fleet-refresh":  newFleetRefreshCmd(flags),
		"fleet-status":   newFleetStatusCmd(flags),
	}
	for name, cmd := range subs {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("%s under verify-mode: %v\noutput: %s", name, err, buf.String())
			}
			// Verify-mode output is a JSON line with verify_noop=true.
			if !strings.Contains(buf.String(), "verify_noop") {
				t.Errorf("%s: expected verify_noop in output, got: %s", name, buf.String())
			}
		})
	}
}

// TestFleetSetup_NoNetwork asserts fleet-setup prints the walkthrough without
// any disk write or network call.
func TestFleetSetup_NoNetwork(t *testing.T) {
	flags := configFlags(t)
	cmd := newFleetSetupCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet-setup: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "developer.tesla.com") {
		t.Errorf("expected developer.tesla.com in walkthrough, got:\n%s", out)
	}
	// Config file should not have been written.
	if _, err := os.Stat(flags.configPath); err == nil {
		t.Errorf("fleet-setup wrote config.toml; expected read-only walkthrough")
	}
}

// TestDecodeJWTClaims covers the audience+scope extraction helper directly.
func TestDecodeJWTClaims(t *testing.T) {
	jwt := mintJWT(t, "https://fleet-api.prd.na.vn.cloud.tesla.com", "openid vehicle_cmds")
	aud, scope, err := decodeJWTClaims(jwt)
	if err != nil {
		t.Fatalf("decodeJWTClaims: %v", err)
	}
	if len(aud) == 0 || !strings.Contains(aud[0], "fleet-api") {
		t.Errorf("audience: got %v", aud)
	}
	if !strings.Contains(scope, "vehicle_cmds") {
		t.Errorf("scope: got %q", scope)
	}
}

// listenerBlocked returns true if the test env can't bind 127.0.0.1:8585 for
// any reason (sandbox, port already grabbed by another process, etc.). Some
// of the OAuth flow tests need exclusive use of the port to exercise the real
// callback handler; on hosts where we can't get it we skip rather than fail.
func listenerBlocked() bool {
	ln, err := net.Listen("tcp", "127.0.0.1:8585")
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// waitForStateInBuf polls a buffer for the `state=...` query param emitted by
// runFleetLoginFlow's printed URL. Returns the extracted state, or "" on
// timeout.
func waitForStateInBuf(t *testing.T, buf *threadSafeBuffer, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s := buf.String()
		if idx := strings.Index(s, "state="); idx >= 0 {
			rest := s[idx+len("state="):]
			end := strings.IndexAny(rest, "&\n\r ")
			if end < 0 {
				end = len(rest)
			}
			st := rest[:end]
			if st != "" {
				// Decode the URL escape so we can re-encode it on the way back.
				dec, err := url.QueryUnescape(st)
				if err == nil {
					return dec
				}
				return st
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ""
}

// threadSafeBuffer is a tiny mutex-wrapped bytes.Buffer so the goroutine that
// runs runFleetLoginFlow can write to stderr while the test goroutine reads.
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
