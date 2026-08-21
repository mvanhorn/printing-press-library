// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
)

// TestDoctorReportsPersistedTokenWithoutBootstrapEnvVars guards the fix for
// doctor reporting "Auth: not configured" / "ERROR missing required" /
// "credentials_location: none" for a user who has a valid persisted
// oauth-token.json bundle but no PELOTON_OAUTH_USERNAME/PASSWORD set —
// exactly the state that previously contradicted doctor's own "API:
// reachable" line and sent users looking for a nonexistent OAuth wrapper.
func TestDoctorReportsPersistedTokenWithoutBootstrapEnvVars(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	bundlePath, err := oauthBundlePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveOAuthBundle(pelotonTokenBundle{
		AccessToken:  "persisted-access",
		RefreshToken: "persisted-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		SessionID:    "persisted-session",
	}); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	t.Setenv("PELOTON_OAUTH_USERNAME", "")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "")

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\noutput: %s", err, out.String())
	}

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}

	auth, _ := report["auth"].(string)
	if auth == "not configured" {
		t.Fatalf(`auth = %q, want something other than "not configured" (a valid persisted token is present)`, auth)
	}
	if envVars, _ := report["env_vars"].(string); len(envVars) >= 5 && envVars[:5] == "ERROR" {
		t.Fatalf("env_vars = %q, want no ERROR (bootstrap creds absent but persisted token present)", envVars)
	}
	credLoc, _ := report["credentials_location"].(string)
	if credLoc == "none" || credLoc == "" {
		t.Fatalf("credentials_location = %q, want the persisted bundle path %q", credLoc, bundlePath)
	}
	if credLoc != bundlePath {
		t.Fatalf("credentials_location = %q, want %q", credLoc, bundlePath)
	}
}

// TestDoctorReportsNoCredentialsAnywhere guards the other half of the
// distinction: with neither bootstrap env vars nor a persisted bundle,
// doctor must still report the true "nothing configured" state.
func TestDoctorReportsNoCredentialsAnywhere(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	t.Setenv("PELOTON_OAUTH_USERNAME", "")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "")

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--home", home, "--json"})
	// doctor still exits 0 for a fully unauthenticated report (it's a
	// diagnostic, not a gate), but tolerate either outcome here — only the
	// report body's content is under test.
	_ = root.Execute()

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	if auth, _ := report["auth"].(string); auth != "not configured" {
		t.Fatalf(`auth = %q, want "not configured"`, auth)
	}
	// This test's zero-credentials setup means flags.newClient() fails at
	// the managed-auth bootstrap check (bootstrapPelotonToken sees both env
	// vars empty and returns before any network call) -- no real HTTP
	// request happens here, unlike a scenario with a valid-looking bundle.
}

// TestDoctorReportsFreshBootstrapEnvVarsBeforeFirstLogin guards a third
// state distinct from both cases above: bootstrap env vars are set (valid
// credentials available) but no live command has run yet, so
// oauth-token.json doesn't exist. Without checking bootstrapEnvSet directly,
// authConfigured fell through to "not configured" here -- a first-run
// instance of the exact contradiction (Auth: FAIL next to Env Vars: OK) this
// whole check exists to eliminate.
func TestDoctorReportsFreshBootstrapEnvVarsBeforeFirstLogin(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	t.Setenv("PELOTON_OAUTH_USERNAME", "fixture-user")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "fixture-password")
	oldClient, oldURL := oauthHTTPClient, oauthTokenURL
	t.Cleanup(func() { oauthHTTPClient, oauthTokenURL = oldClient, oldURL })
	oauthHTTPClient = server.Client()
	oauthTokenURL = server.URL

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\noutput: %s", err, out.String())
	}

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	auth, _ := report["auth"].(string)
	if auth == "not configured" {
		t.Fatalf(`auth = %q, want something other than "not configured" (bootstrap env vars are set)`, auth)
	}
	if envVars, _ := report["env_vars"].(string); len(envVars) >= 5 && envVars[:5] == "ERROR" {
		t.Fatalf("env_vars = %q, want no ERROR (bootstrap env vars are set)", envVars)
	}
	// config.Load() sets cfg.CredentialSource from the env var itself in
	// this scenario, independent of the bundle-file check this test targets
	// -- just confirm it's not the "nothing anywhere" sentinel.
	if credLoc, _ := report["credentials_location"].(string); credLoc == "none" || credLoc == "" {
		t.Fatalf(`credentials_location = %q, want a non-empty, non-"none" source (bootstrap env vars are set)`, credLoc)
	}
}

// TestDoctorSurfacesBundleCheckErrorDistinctFromNoCredentials guards a
// code-review finding: collectCredentialsLocationReport (doctor.go)
// collapsed a real pelotonPersistedBundleStatus() error into the same
// "credentials_location": "none" result as "genuinely no credentials",
// while the auth-check block earlier in the same file (bundleErr, around
// line 159) already surfaces the identical error distinctly. A bundle file
// that exists but fails its own validity check (here: overly-permissive
// file mode, which loadOAuthBundle rejects) must report an "unknown
// (checking persisted bundle failed: ...)" value, not silently look
// identical to "no bundle at all".
func TestDoctorSurfacesBundleCheckErrorDistinctFromNoCredentials(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	t.Setenv("PELOTON_OAUTH_USERNAME", "")
	t.Setenv("PELOTON_OAUTH_PASSWORD", "")

	bundlePath, err := oauthBundlePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bundlePath), 0o700); err != nil {
		t.Fatal(err)
	}
	// Bypass saveOAuthBundle (which writes with a restrictive mode) and
	// write directly with a world-readable mode -- loadOAuthBundle
	// rejects this, producing a real error distinct from "file absent".
	if err := os.WriteFile(bundlePath, []byte(`{"access_token":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--home", home, "--json"})
	_ = root.Execute()

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	credLoc, _ := report["credentials_location"].(string)
	if credLoc == "none" {
		t.Fatal(`credentials_location = "none", want a distinct error indication -- a real check failure was silently reported identically to "no credentials anywhere"`)
	}
	if !strings.Contains(credLoc, "checking persisted bundle failed") {
		t.Fatalf("credentials_location = %q, want it to name the check failure", credLoc)
	}
}
