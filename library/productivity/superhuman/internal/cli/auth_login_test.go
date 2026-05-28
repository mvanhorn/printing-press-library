// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
)

// mockChromeForLogin stands up an httptest.Server that mimics Chrome's
// /json/version + /json HTTP endpoints. Mirrors the helper in
// internal/auth/cdp_test.go so the CLI-level test surface stays self-
// contained.
func mockChromeForLogin(t *testing.T, tabsBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"Browser":"Chrome/120"}`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, tabsBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// pinFactoryToServer wires the package-level authLoginCDPFactory to a
// CDPClient whose HTTP transport routes every 127.0.0.1:<port> request to
// the httptest server. Tests call this so DiscoverPort/ListTabs are
// exercised against the mock without touching real ports.
func pinFactoryToServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	target, _ := url.Parse(srv.URL)
	prev := authLoginCDPFactory
	authLoginCDPFactory = func(port int) *auth.CDPClient {
		return &auth.CDPClient{
			Port: 9222, // any port — DialContext below shunts it to srv
			HTTPClient: &http.Client{
				Timeout: 2 * time.Second,
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						var d net.Dialer
						return d.DialContext(ctx, network, target.Host)
					},
				},
			},
		}
	}
	t.Cleanup(func() {
		authLoginCDPFactory = prev
	})
}

// stubExtract wires authLoginExtractFn to return fixed tokens for the next
// extract call. Returns a function that unwires the stub.
func stubExtract(t *testing.T, tokens *auth.ExtractedTokens, err error) {
	t.Helper()
	prev := authLoginExtractFn
	authLoginExtractFn = func(ctx context.Context, c *auth.CDPClient, tab auth.Tab) (*auth.ExtractedTokens, error) {
		return tokens, err
	}
	t.Cleanup(func() {
		authLoginExtractFn = prev
	})
}

// withConfigPath returns a config file path inside a temp dir and the
// resulting token-store path. The CLI's --config flag points at this so
// Config.TokenStorePath() resolves to <tempdir>/tokens.json — no
// $XDG_CONFIG_HOME hacks needed.
func withConfigPath(t *testing.T) (configPath, tokenStorePath string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.toml")
	tokenStorePath = filepath.Join(dir, "tokens.json")
	return configPath, tokenStorePath
}

// executeCmd runs the root command with the given args and returns stdout,
// stderr, and the run error. The root command is built fresh per call so
// shared package state (flags) doesn't leak between scenarios.
//
// stdin is intentionally injected as an empty *bytes.Buffer (not os.Stdin)
// so isTerminalReader returns false in test runs. Several destructive
// commands (drafts discard, threads trash, etc.) gate confirmation on a
// TTY stdin; without this injection, tests would inherit the test
// runner's potentially-TTY stdin and prompts would fire instead of the
// usage error the tests assert on.
func executeCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf, inBuf bytes.Buffer
	flags := &rootFlags{}
	rootCmd := newRootCmd(flags)
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetIn(&inBuf)
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// fakeTokens returns a populated ExtractedTokens for happy-path scenarios.
func fakeTokens(email string) *auth.ExtractedTokens {
	now := time.Now().UnixMilli()
	return &auth.ExtractedTokens{
		Email:              email,
		IDToken:            "id-token-" + email,
		IDTokenExpires:     now + int64((time.Hour).Milliseconds()),
		RefreshToken:       "refresh-token-" + email,
		AccessToken:        "access-token-" + email,
		AccessTokenExpires: now + int64((time.Hour).Milliseconds()),
		UserID:             "user_ABCDEFG1234567890",
		UserPrefix:         "1234",
		UserExternalID:     "user_ABCDEFG1234567890",
		DeviceID:           "dev_test",
		Provider:           "google",
	}
}

// Scenario 1: `auth login --chrome` with single Superhuman tab and no
// --account: extracts, saves, prints success.
func TestAuthLogin_SingleTab_NoAccount_Saves(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	body := `[
		{"id":"A","type":"page","url":"https://mail.superhuman.com/single@example.com/threads","webSocketDebuggerUrl":"ws://x/A"}
	]`
	srv := mockChromeForLogin(t, body)
	pinFactoryToServer(t, srv)
	stubExtract(t, fakeTokens("single@example.com"), nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--chrome")
	if err != nil {
		t.Fatalf("auth login: %v", err)
	}
	if !strings.Contains(stdout, "Saved single@example.com") {
		t.Fatalf("expected success line in stdout, got: %s", stdout)
	}

	// Verify the token store now contains the account.
	store := auth.NewStoreAt(tokenStorePath)
	p, loadErr := store.Load()
	if loadErr != nil {
		t.Fatalf("load store: %v", loadErr)
	}
	if _, ok := p.Accounts["single@example.com"]; !ok {
		t.Fatalf("expected single@example.com in store, got %v", p.Accounts)
	}
}

// Scenario 2: `auth login --chrome` with two Superhuman tabs and no
// --account: errors with list of emails.
func TestAuthLogin_MultiTab_NoAccount_Errors(t *testing.T) {
	configPath, _ := withConfigPath(t)
	body := `[
		{"id":"A","type":"page","url":"https://mail.superhuman.com/foo@example.com/threads","webSocketDebuggerUrl":"ws://x/A"},
		{"id":"B","type":"page","url":"https://mail.superhuman.com/bar@example.com/threads","webSocketDebuggerUrl":"ws://x/B"}
	]`
	srv := mockChromeForLogin(t, body)
	pinFactoryToServer(t, srv)

	_, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--chrome")
	if err == nil {
		t.Fatalf("expected error for multi-tab no-account case")
	}
	msg := err.Error()
	if !strings.Contains(msg, "multiple") {
		t.Fatalf("expected 'multiple' in error, got: %s", msg)
	}
	if !strings.Contains(msg, "foo@example.com") || !strings.Contains(msg, "bar@example.com") {
		t.Fatalf("expected both emails in error, got: %s", msg)
	}
}

// Scenario 3: `auth login --chrome --account <email>` selects correctly
// even when multiple tabs are open.
func TestAuthLogin_MultiTab_AccountSelects(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	body := `[
		{"id":"A","type":"page","url":"https://mail.superhuman.com/foo@example.com/threads","webSocketDebuggerUrl":"ws://x/A"},
		{"id":"B","type":"page","url":"https://mail.superhuman.com/bar@example.com/threads","webSocketDebuggerUrl":"ws://x/B"}
	]`
	srv := mockChromeForLogin(t, body)
	pinFactoryToServer(t, srv)
	stubExtract(t, fakeTokens("bar@example.com"), nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "--account", "bar@example.com", "auth", "login", "--chrome")
	if err != nil {
		t.Fatalf("auth login --account: %v", err)
	}
	if !strings.Contains(stdout, "Saved bar@example.com") {
		t.Fatalf("expected save line for bar@example.com, got: %s", stdout)
	}
	store := auth.NewStoreAt(tokenStorePath)
	p, _ := store.Load()
	if _, ok := p.Accounts["bar@example.com"]; !ok {
		t.Fatalf("expected bar@example.com in store; got %v", p.Accounts)
	}
	if _, ok := p.Accounts["foo@example.com"]; ok {
		t.Fatalf("did not expect foo@example.com to be saved")
	}
}

// Scenario 4: `auth login --chrome` with no Chrome running. We pin the
// CDPClient factory to a CDPClient with HTTPClient that always fails the
// connection, so DiscoverPort returns ErrChromeNotRunning. The command
// prints the relaunch hint and exits with the typed setup-required code.
func TestAuthLogin_NoChrome_ExitsTyped(t *testing.T) {
	configPath, _ := withConfigPath(t)

	// Force a closed-connection transport: dial localhost:1 (reserved, never
	// listening) so every attempt connects to /dev/null.
	prev := authLoginCDPFactory
	authLoginCDPFactory = func(port int) *auth.CDPClient {
		return &auth.CDPClient{
			Port: 9222,
			HTTPClient: &http.Client{
				Timeout: 200 * time.Millisecond,
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						// Dial an unreachable port so every probe is conn-refused.
						var d net.Dialer
						return d.DialContext(ctx, network, "127.0.0.1:1")
					},
				},
			},
		}
	}
	t.Cleanup(func() { authLoginCDPFactory = prev })

	_, stderr, err := executeCmd(t, "--config", configPath, "auth", "login", "--chrome")
	if err == nil {
		t.Fatalf("expected error when Chrome unreachable")
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d, want 4 (setup-required)", got)
	}
	// The relaunch hint MUST be printed to stderr so the user can copy-paste
	// the recommended command (R9). The literal "--remote-debugging-port"
	// substring is the load-bearing part of the hint.
	if !strings.Contains(stderr, "--remote-debugging-port") {
		t.Fatalf("expected relaunch hint in stderr, got: %s", stderr)
	}
}

// Scenario 5: `auth login --chrome --auto-launch-chrome` with no Chrome
// running, under PRINTING_PRESS_VERIFY=1: prints "would launch:" and exits 0.
func TestAuthLogin_AutoLaunch_VerifyShortCircuits(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	configPath, _ := withConfigPath(t)

	// Same unreachable-transport trick as scenario 4 so DiscoverPort fails
	// with ErrChromeNotRunning, triggering the --auto-launch-chrome branch.
	prev := authLoginCDPFactory
	authLoginCDPFactory = func(port int) *auth.CDPClient {
		return &auth.CDPClient{
			Port: 9222,
			HTTPClient: &http.Client{
				Timeout: 200 * time.Millisecond,
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						var d net.Dialer
						return d.DialContext(ctx, network, "127.0.0.1:1")
					},
				},
			},
		}
	}
	t.Cleanup(func() { authLoginCDPFactory = prev })

	_, stderr, err := executeCmd(t, "--config", configPath, "auth", "login", "--chrome", "--auto-launch-chrome")
	if err != nil {
		t.Fatalf("verify-mode auto-launch must exit 0, got: %v", err)
	}
	if !strings.Contains(stderr, "would launch:") {
		t.Fatalf("expected 'would launch:' in stderr under verify mode, got: %s", stderr)
	}
}

// Scenario 6: `auth status` with 0 accounts: prints the actionable
// "Not authenticated" line pointing at auth login --chrome.
func TestAuthStatus_NoAccounts_PrintsHint(t *testing.T) {
	configPath, _ := withConfigPath(t)

	// Ensure no SUPERHUMAN_JWT env var bleeds in from the test runner; the
	// fallback would otherwise mask the "no accounts" message.
	t.Setenv("SUPERHUMAN_JWT", "")

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "status")
	if err == nil {
		t.Fatalf("expected non-nil error so auth-failure exit code is set")
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d, want 4 (auth)", got)
	}
	if !strings.Contains(stdout, "auth login --chrome") {
		t.Fatalf("expected hint pointing at auth login --chrome, got: %s", stdout)
	}
}

// seedStore writes a tokens.json file with the two account entries used by
// the multi-account auth-status tests. Token expiries are spaced so the
// classifier emits distinct status labels.
func seedStore(t *testing.T, tokenStorePath string) {
	t.Helper()
	dir := filepath.Dir(tokenStorePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	now := time.Now().UnixMilli()
	p := auth.PersistedTokens{
		Version: auth.CurrentSchemaVersion,
		Accounts: map[string]auth.AccountTokens{
			"a@example.com": {
				Type:         "google",
				RefreshToken: "refresh-a",
				SuperhumanToken: auth.SuperhumanToken{
					Token:   "id-a",
					Expires: now + int64((47 * time.Minute).Milliseconds()),
				},
				LastUsedAt: now,
			},
			"b@example.com": {
				Type:         "google",
				RefreshToken: "refresh-b",
				SuperhumanToken: auth.SuperhumanToken{
					Token:   "id-b",
					Expires: now - int64((12 * time.Hour).Milliseconds()),
				},
				LastUsedAt: now - 1000,
			},
		},
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(tokenStorePath, data, 0o600); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

// Scenario 7: `auth status` with 2 accounts lists both with relative
// expiry. We assert both emails appear; relative formatting is exercised
// by classifyAccount's pure-function tests.
func TestAuthStatus_TwoAccounts_ListsBoth(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedStore(t, tokenStorePath)

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "status")
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}
	if !strings.Contains(stdout, "a@example.com") {
		t.Fatalf("expected a@example.com in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "b@example.com") {
		t.Fatalf("expected b@example.com in output, got: %s", stdout)
	}
	// b@ expired 12h ago should land on the FAIL row.
	if !strings.Contains(stdout, "expired") {
		t.Fatalf("expected 'expired' on b@example.com row, got: %s", stdout)
	}
	// a@ expires in ~47m should still be labelled with "expires in".
	if !strings.Contains(stdout, "expires in") {
		t.Fatalf("expected 'expires in' on a@example.com row, got: %s", stdout)
	}
}

// Scenario 8: `auth status --json` returns an array with the expected
// per-account fields.
func TestAuthStatus_JSON_ReturnsArray(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	t.Setenv("SUPERHUMAN_JWT", "")
	seedStore(t, tokenStorePath)

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "auth", "status")
	if err != nil {
		t.Fatalf("auth status --json: %v", err)
	}

	var rows []map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &rows); jerr != nil {
		t.Fatalf("parse JSON output: %v\nbody: %s", jerr, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d (%v)", len(rows), rows)
	}
	// Required keys per plan: email, expires_at, refresh_expires_at, status.
	for _, r := range rows {
		for _, key := range []string{"email", "expires_at", "refresh_expires_at", "status"} {
			if _, ok := r[key]; !ok {
				t.Fatalf("row missing %q: %v", key, r)
			}
		}
	}
}

// ---------------------------------------------------------------------
// auth login --disk: disk-auth path. Tests inject fake localStorage,
// cookies, and refresh results via the authLoginRead*/Refresh test seams
// so the entire RunE path is exercised without touching real Chrome or
// the user's keychain.
// ---------------------------------------------------------------------

// stubDiskAuth wires the disk-auth test seams to canned fixtures for a
// single test scope. Returns the unwire function via t.Cleanup so each
// scenario starts from a known state.
func stubDiskAuth(t *testing.T, localStorage, cookies map[string]string, refreshResults map[string]*auth.CookieAuthResult, refreshErr error) {
	t.Helper()
	prevLS := authLoginReadLocalStorageFn
	prevCk := authLoginReadCookiesFn
	prevRf := authLoginRefreshFn
	authLoginReadLocalStorageFn = func(profileDir string) (map[string]string, error) {
		if localStorage == nil {
			return nil, fmt.Errorf("no localStorage fixture wired")
		}
		out := make(map[string]string, len(localStorage))
		for k, v := range localStorage {
			out[k] = v
		}
		return out, nil
	}
	authLoginReadCookiesFn = func(host string) (map[string]string, error) {
		if cookies == nil {
			return nil, fmt.Errorf("no cookies fixture wired for host %q", host)
		}
		out := make(map[string]string, len(cookies))
		for k, v := range cookies {
			out[k] = v
		}
		return out, nil
	}
	authLoginRefreshFn = func(ctx context.Context, email, googleID string) (*auth.CookieAuthResult, error) {
		if refreshErr != nil {
			return nil, refreshErr
		}
		if r, ok := refreshResults[googleID]; ok {
			return r, nil
		}
		return nil, fmt.Errorf("no refresh fixture for googleID=%s", googleID)
	}
	t.Cleanup(func() {
		authLoginReadLocalStorageFn = prevLS
		authLoginReadCookiesFn = prevCk
		authLoginRefreshFn = prevRf
	})
}

// fakeCookieAuthResult returns a populated CookieAuthResult for the
// disk-auth happy path. Fields mirror what RefreshFromChromeCookies
// produces against the real backend.
func fakeCookieAuthResult(email, googleID string) *auth.CookieAuthResult {
	now := time.Now().UnixMilli()
	return &auth.CookieAuthResult{
		Email:          email,
		GoogleID:       googleID,
		IDToken:        "id-token-" + email,
		IDTokenExpires: now + int64((time.Hour).Milliseconds()),
		AccessToken:    "access-token-" + email,
		ExternalID:     "user_external_" + googleID,
		Scope:          "email profile",
		DeviceID:       "device-" + email,
	}
}

// Scenario disk-1: single account in localStorage + matching cookie ->
// one upsert, "Captured 1 account(s)" message, store contains the email.
func TestAuthLoginDisk_SingleAccount_Captures(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	ls := map[string]string{
		"single@example.com:id": `"111111111111111111"`,
		"deviceId":              `"device-single"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"111111111111111111": "session-cookie-value",
	}
	refresh := map[string]*auth.CookieAuthResult{
		"111111111111111111": fakeCookieAuthResult("single@example.com", "111111111111111111"),
	}
	stubDiskAuth(t, ls, cookies, refresh, nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--disk")
	if err != nil {
		t.Fatalf("auth login --disk: %v", err)
	}
	if !strings.Contains(stdout, "Captured 1 account") {
		t.Fatalf("expected 'Captured 1 account' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "single@example.com") {
		t.Fatalf("expected single@example.com in stdout, got: %s", stdout)
	}

	store := auth.NewStoreAt(tokenStorePath)
	p, loadErr := store.Load()
	if loadErr != nil {
		t.Fatalf("load store: %v", loadErr)
	}
	got, ok := p.Accounts["single@example.com"]
	if !ok {
		t.Fatalf("expected single@example.com in store, got %v", p.Accounts)
	}
	if got.SuperhumanToken.Token != "id-token-single@example.com" {
		t.Fatalf("unexpected token persisted: %q", got.SuperhumanToken.Token)
	}
	if got.UserExternalID != "user_external_111111111111111111" {
		t.Fatalf("unexpected externalId persisted: %q", got.UserExternalID)
	}
}

// Scenario disk-2: two accounts in localStorage + two cookies -> two
// upserts, summary lists both, store contains both.
func TestAuthLoginDisk_MultiAccount_CapturesAll(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	ls := map[string]string{
		"user2@example.com:id": `"111111111111111111"`,
		"user@example.com:id":   `"222222222222222222"`,
		"deviceId":              `"device-shared"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"111111111111111111": "session-personal",
		"222222222222222222": "session-work",
	}
	refresh := map[string]*auth.CookieAuthResult{
		"111111111111111111": fakeCookieAuthResult("user2@example.com", "111111111111111111"),
		"222222222222222222": fakeCookieAuthResult("user@example.com", "222222222222222222"),
	}
	stubDiskAuth(t, ls, cookies, refresh, nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--disk")
	if err != nil {
		t.Fatalf("auth login --disk: %v", err)
	}
	if !strings.Contains(stdout, "Captured 2 account") {
		t.Fatalf("expected 'Captured 2 account' in stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "user2@example.com") || !strings.Contains(stdout, "user@example.com") {
		t.Fatalf("expected both emails in stdout, got: %s", stdout)
	}

	store := auth.NewStoreAt(tokenStorePath)
	p, _ := store.Load()
	if _, ok := p.Accounts["user2@example.com"]; !ok {
		t.Fatalf("expected user2@example.com in store, got %v", p.Accounts)
	}
	if _, ok := p.Accounts["user@example.com"]; !ok {
		t.Fatalf("expected user@example.com in store, got %v", p.Accounts)
	}
}

// Scenario disk-3: --account filter captures only the named account
// even when two are available.
func TestAuthLoginDisk_AccountFilter_CapturesOnlyNamed(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	ls := map[string]string{
		"user2@example.com:id": `"111111111111111111"`,
		"user@example.com:id":   `"222222222222222222"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"111111111111111111": "session-personal",
		"222222222222222222": "session-work",
	}
	refresh := map[string]*auth.CookieAuthResult{
		"111111111111111111": fakeCookieAuthResult("user2@example.com", "111111111111111111"),
		"222222222222222222": fakeCookieAuthResult("user@example.com", "222222222222222222"),
	}
	stubDiskAuth(t, ls, cookies, refresh, nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "--account", "user@example.com", "auth", "login", "--disk")
	if err != nil {
		t.Fatalf("auth login --disk --account: %v", err)
	}
	if !strings.Contains(stdout, "Captured 1 account") {
		t.Fatalf("expected 'Captured 1 account', got: %s", stdout)
	}
	if !strings.Contains(stdout, "user@example.com") {
		t.Fatalf("expected work account in stdout, got: %s", stdout)
	}

	store := auth.NewStoreAt(tokenStorePath)
	p, _ := store.Load()
	if _, ok := p.Accounts["user@example.com"]; !ok {
		t.Fatalf("expected user@example.com in store, got %v", p.Accounts)
	}
	if _, ok := p.Accounts["user2@example.com"]; ok {
		t.Fatalf("did not expect user2@example.com to be saved")
	}
}

// Scenario disk-4: --account points at an email that isn't in Chrome
// -> typed error listing the available emails so the user can copy
// the right one.
func TestAuthLoginDisk_AccountFilter_UnknownEmailErrors(t *testing.T) {
	configPath, _ := withConfigPath(t)
	ls := map[string]string{
		"user2@example.com:id": `"111111111111111111"`,
		"user@example.com:id":   `"222222222222222222"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"111111111111111111": "session-personal",
		"222222222222222222": "session-work",
	}
	refresh := map[string]*auth.CookieAuthResult{
		"111111111111111111": fakeCookieAuthResult("user2@example.com", "111111111111111111"),
		"222222222222222222": fakeCookieAuthResult("user@example.com", "222222222222222222"),
	}
	stubDiskAuth(t, ls, cookies, refresh, nil)

	_, _, err := executeCmd(t, "--config", configPath, "--account", "nobody@example.com", "auth", "login", "--disk")
	if err == nil {
		t.Fatalf("expected error for unknown --account")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not found in Chrome cookies") {
		t.Fatalf("expected 'not found in Chrome cookies' in error, got: %s", msg)
	}
	if !strings.Contains(msg, "user2@example.com") || !strings.Contains(msg, "user@example.com") {
		t.Fatalf("expected available emails listed in error, got: %s", msg)
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d, want 4 (auth)", got)
	}
}

// Scenario disk-5: no cookies for accounts.superhuman.com -> typed
// error with the "log in to mail.superhuman.com" hint.
func TestAuthLoginDisk_NoCookies_ErrorsWithHint(t *testing.T) {
	configPath, _ := withConfigPath(t)
	ls := map[string]string{
		"user@example.com:id": `"222222222222222222"`,
	}
	// Cookie reader returns the typed "no cookies" error verbatim from
	// the real DecryptedChromeCookies; stub returns nil + error.
	prevCk := authLoginReadCookiesFn
	authLoginReadCookiesFn = func(host string) (map[string]string, error) {
		return nil, fmt.Errorf("no cookies found for host %q (have you logged in to Superhuman in Chrome?)", host)
	}
	prevLS := authLoginReadLocalStorageFn
	authLoginReadLocalStorageFn = func(profileDir string) (map[string]string, error) {
		return ls, nil
	}
	t.Cleanup(func() {
		authLoginReadCookiesFn = prevCk
		authLoginReadLocalStorageFn = prevLS
	})

	_, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--disk")
	if err == nil {
		t.Fatalf("expected error when no cookies present")
	}
	msg := err.Error()
	if !strings.Contains(msg, "log in to mail.superhuman.com") {
		t.Fatalf("expected actionable hint in error, got: %s", msg)
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode = %d, want 4 (auth)", got)
	}
}

// Scenario disk-6: PRINTING_PRESS_VERIFY=1 short-circuits the HTTP
// refresh and writes placeholder tokens. No network is touched.
func TestAuthLoginDisk_VerifyEnv_WritesPlaceholderTokens(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	configPath, tokenStorePath := withConfigPath(t)
	ls := map[string]string{
		"user@example.com:id": `"222222222222222222"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"222222222222222222": "session-work",
	}
	// Refresh stub returns an error so we'd notice if verify-mode ever
	// fell through to the real call. The captureDiskAccount short-circuit
	// must beat this.
	stubDiskAuth(t, ls, cookies, nil, fmt.Errorf("would-have-called-real-refresh"))

	_, _, err := executeCmd(t, "--config", configPath, "auth", "login", "--disk")
	if err != nil {
		t.Fatalf("verify-mode auth login --disk: %v", err)
	}
	store := auth.NewStoreAt(tokenStorePath)
	p, _ := store.Load()
	got, ok := p.Accounts["user@example.com"]
	if !ok {
		t.Fatalf("expected user@example.com in store under verify mode, got %v", p.Accounts)
	}
	if got.SuperhumanToken.Token != "verify-mode-id-token" {
		t.Fatalf("expected placeholder token under verify mode, got %q", got.SuperhumanToken.Token)
	}
}

// Scenario disk-7: no flag at all -> route defaults to --disk so a
// user running `auth login` (the obvious incantation) gets the modern
// path without having to remember the flag.
func TestAuthLogin_DefaultsToDisk(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	ls := map[string]string{
		"default@example.com:id": `"333333333333333333"`,
	}
	cookies := map[string]string{
		"csrf":               "csrf-value",
		"333333333333333333": "session-default",
	}
	refresh := map[string]*auth.CookieAuthResult{
		"333333333333333333": fakeCookieAuthResult("default@example.com", "333333333333333333"),
	}
	stubDiskAuth(t, ls, cookies, refresh, nil)

	stdout, _, err := executeCmd(t, "--config", configPath, "auth", "login")
	if err != nil {
		t.Fatalf("auth login (no flag): %v", err)
	}
	if !strings.Contains(stdout, "Captured 1 account") {
		t.Fatalf("expected 'Captured 1 account' in stdout, got: %s", stdout)
	}
	store := auth.NewStoreAt(tokenStorePath)
	p, _ := store.Load()
	if _, ok := p.Accounts["default@example.com"]; !ok {
		t.Fatalf("expected default@example.com in store, got %v", p.Accounts)
	}
}

// ---------------------------------------------------------------------
// Pure-function unit tests for classifyAccount: every branch of the
// status taxonomy. Cheap, no I/O, exercised on every test run.
// ---------------------------------------------------------------------

func TestClassifyAccount_AllBranches(t *testing.T) {
	now := time.Now().UnixMilli()
	hour := int64((time.Hour).Milliseconds())
	cases := []struct {
		name            string
		expiresMs       int64
		hasRefreshToken bool
		wantStatus      string
	}{
		{"valid: 1h ahead, has refresh", now + hour, true, "valid"},
		{"expiring soon: 2m ahead, has refresh", now + int64((2 * time.Minute).Milliseconds()), true, "expiring_soon"},
		{"expired: 1h ago, has refresh", now - hour, true, "expired"},
		{"refresh_expired: 1h ago, no refresh", now - hour, false, "refresh_expired"},
		{"valid no-refresh: still in window", now + hour, false, "valid"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classifyAccount(tc.expiresMs, now, tc.hasRefreshToken)
			if got != tc.wantStatus {
				t.Fatalf("status = %q, want %q", got, tc.wantStatus)
			}
		})
	}
}
