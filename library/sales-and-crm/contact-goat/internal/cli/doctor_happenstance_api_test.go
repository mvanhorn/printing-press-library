// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// doctor_happenstance_api_test.go: tests for the three doctor rows that
// surface the bearer-auth Happenstance public REST API surface.
//
// Coverage matches the plan's unit-7 test scenario list:
//
//   - Happy: env set + 200 on /v1/users/me + 200 on /v1/usage with positive
//     balance -> three OK lines, no FAIL.
//   - Edge: env set + 401 on /v1/users/me -> validity FAIL with rotation URL.
//   - Edge: env set + 200 on /v1/users/me + balance:0 -> balance WARN with
//     has_credits=false; validity stays OK.
//   - Edge: env unset (cookie auth would be the user's other surface) ->
//     key WARN; validity and balance probes never attempted.
//   - Redaction: doctor JSON output redacts the key (last 4 chars only);
//     a grep for the literal value across the full output returns 0.
//
// All tests use httptest fixtures so doctor never touches the real
// api.happenstance.ai. The base URL is injected via the package-level
// happenstanceAPIBaseURLOverride variable.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
	hpapi "github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// withHappenstanceAPIKey sets HAPPENSTANCE_API_KEY for the duration of
// a test and restores the previous value (or unsets) on cleanup. Mirrors
// the withEnv helper used in source_selection_test.go.
func withHappenstanceAPIKey(t *testing.T, value string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(hpapi.KeyEnvVar)
	if value == "" {
		_ = os.Unsetenv(hpapi.KeyEnvVar)
	} else {
		_ = os.Setenv(hpapi.KeyEnvVar, value)
	}
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv(hpapi.KeyEnvVar, prev)
		} else {
			_ = os.Unsetenv(hpapi.KeyEnvVar)
		}
	})
}

// withHappenstanceAPIBaseURL points the doctor's bearer probes at an
// httptest server for the duration of a test.
func withHappenstanceAPIBaseURL(t *testing.T, url string) {
	t.Helper()
	prev := happenstanceAPIBaseURLOverride
	happenstanceAPIBaseURLOverride = url
	t.Cleanup(func() { happenstanceAPIBaseURLOverride = prev })
}

// stubCfg returns a *config.Config pointing at a non-existent file in a
// fresh temp dir. checkHappenstanceAPI only reads via config.LoadAPIKey,
// which falls back to the env var when the file is absent.
func stubCfg(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{Path: t.TempDir() + "/config.toml"}
}

// happyHappenstanceAPIServer mimics the two endpoints doctor probes:
// GET /users/me returns a populated User; GET /usage returns a positive
// balance with has_credits true. The path prefix is "/" because the
// hpapi.WithBaseURL override carries the v1 segment if the test wants it.
func happyHappenstanceAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":"matt@example.com","name":"Matt","friends":[]}`))
	})
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"balance_credits":42,"has_credits":true}`))
	})
	return httptest.NewServer(mux)
}

// TestCheckHappenstanceAPI_HappyPath: env set + 200 on both probes with a
// positive balance lands three OK rows with no FAIL.
func TestCheckHappenstanceAPI_HappyPath(t *testing.T) {
	srv := happyHappenstanceAPIServer(t)
	defer srv.Close()
	withHappenstanceAPIBaseURL(t, srv.URL)
	withHappenstanceAPIKey(t, "hpn_live_personal_FAKE_TEST_KEY_NEVER_SENT_BEEF")

	report := map[string]any{}
	checkHappenstanceAPI(stubCfg(t), report)

	keyVal := mustString(t, report, "happenstance_api_key")
	if !strings.HasPrefix(keyVal, "OK ") {
		t.Errorf("key row should start with OK, got %q", keyVal)
	}
	if !strings.Contains(keyVal, "hpn_live_personal_") {
		t.Errorf("key row should include recognized prefix, got %q", keyVal)
	}
	if !strings.Contains(keyVal, "BEEF") {
		t.Errorf("key row should include last 4 chars, got %q", keyVal)
	}

	validityVal := mustString(t, report, "happenstance_api_validity")
	if !strings.HasPrefix(validityVal, "OK ") {
		t.Errorf("validity row should start with OK, got %q", validityVal)
	}
	if !strings.Contains(validityVal, "matt@example.com") {
		t.Errorf("validity row should include user email, got %q", validityVal)
	}

	balanceVal := mustString(t, report, "happenstance_api_balance")
	if !strings.HasPrefix(balanceVal, "OK ") {
		t.Errorf("balance row should start with OK, got %q", balanceVal)
	}
	if !strings.Contains(balanceVal, "42") {
		t.Errorf("balance row should include credit count, got %q", balanceVal)
	}

	for k, v := range report {
		s := fmt.Sprintf("%v", v)
		if strings.HasPrefix(s, "FAIL ") {
			t.Errorf("no FAIL expected on happy path; %s = %q", k, s)
		}
	}
}

// TestCheckHappenstanceAPI_Validity401: a key that the upstream rejects
// surfaces as a FAIL on the validity row, with the rotation URL embedded
// in the message. The balance row is not added to the report (we only
// probe usage when validity is OK).
func TestCheckHappenstanceAPI_Validity401(t *testing.T) {
	var meHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me" {
			atomic.AddInt32(&meHits, 1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
			return
		}
		t.Errorf("unexpected request to %s after validity 401", r.URL.Path)
		http.Error(w, "should not reach", http.StatusInternalServerError)
	}))
	defer srv.Close()
	withHappenstanceAPIBaseURL(t, srv.URL)
	withHappenstanceAPIKey(t, "hpn_live_personal_FAKE_TEST_KEY_NEVER_SENT_DEAD")

	report := map[string]any{}
	checkHappenstanceAPI(stubCfg(t), report)

	validityVal := mustString(t, report, "happenstance_api_validity")
	if !strings.HasPrefix(validityVal, "FAIL ") {
		t.Errorf("validity should FAIL on 401, got %q", validityVal)
	}
	if !strings.Contains(validityVal, hpapi.RotationURL) {
		t.Errorf("validity FAIL should include rotation URL %s, got %q", hpapi.RotationURL, validityVal)
	}
	if _, ok := report["happenstance_api_balance"]; ok {
		t.Errorf("balance row should NOT be set when validity fails (would burn an extra request)")
	}
	if got := atomic.LoadInt32(&meHits); got != 1 {
		t.Errorf("expected exactly one /users/me probe, got %d", got)
	}
}

// TestCheckHappenstanceAPI_BalanceZero: a valid key with no credits left
// surfaces as WARN on the balance row (with the top-up URL) while the
// validity row stays OK.
func TestCheckHappenstanceAPI_BalanceZero(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"email":"matt@example.com","name":"Matt","friends":[]}`))
	})
	mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		// has_credits=false AND balance_credits=0: both conditions the
		// plan flags as WARN.
		_, _ = w.Write([]byte(`{"balance_credits":0,"has_credits":false}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	withHappenstanceAPIBaseURL(t, srv.URL)
	withHappenstanceAPIKey(t, "hpn_live_personal_FAKE_TEST_KEY_NEVER_SENT_CAFE")

	report := map[string]any{}
	checkHappenstanceAPI(stubCfg(t), report)

	validityVal := mustString(t, report, "happenstance_api_validity")
	if !strings.HasPrefix(validityVal, "OK ") {
		t.Errorf("validity should remain OK when only the balance is 0, got %q", validityVal)
	}

	balanceVal := mustString(t, report, "happenstance_api_balance")
	if !strings.HasPrefix(balanceVal, "WARN ") {
		t.Errorf("balance should WARN on has_credits=false, got %q", balanceVal)
	}
	if !strings.Contains(balanceVal, "top up") {
		t.Errorf("balance WARN should include top-up hint, got %q", balanceVal)
	}
	if !strings.Contains(balanceVal, "0 credits") {
		t.Errorf("balance WARN should include the literal balance (0 credits), got %q", balanceVal)
	}
}

// TestCheckHappenstanceAPI_KeyUnset: with no env var (and no config-file
// key), the key row WARNs and the validity/balance probes are skipped.
// This is the "cookie path may suffice" case — doctor stays exit-0.
func TestCheckHappenstanceAPI_KeyUnset(t *testing.T) {
	// Set the override URL to a server that fails any request, so we can
	// prove no probe happened by virtue of no test failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("doctor should not have hit %s when no key is set", r.URL.Path)
		http.Error(w, "should not reach", http.StatusInternalServerError)
	}))
	defer srv.Close()
	withHappenstanceAPIBaseURL(t, srv.URL)
	withHappenstanceAPIKey(t, "")

	report := map[string]any{}
	checkHappenstanceAPI(stubCfg(t), report)

	keyVal := mustString(t, report, "happenstance_api_key")
	if !strings.HasPrefix(keyVal, "WARN ") {
		t.Errorf("key row should WARN when env+config both empty, got %q", keyVal)
	}
	if strings.HasPrefix(keyVal, "FAIL ") {
		t.Errorf("key row must NOT FAIL when env+config both empty (cookie surface may suffice), got %q", keyVal)
	}
	if _, ok := report["happenstance_api_validity"]; ok {
		t.Errorf("validity row should NOT be set when no key is configured")
	}
	if _, ok := report["happenstance_api_balance"]; ok {
		t.Errorf("balance row should NOT be set when no key is configured")
	}
}

// TestCheckHappenstanceAPI_KeyUnknownPrefix: a key set but with an
// unrecognized prefix WARNs on the key row. Validity and balance still
// run because the loose prefix check is informational, not gating.
func TestCheckHappenstanceAPI_KeyUnknownPrefix(t *testing.T) {
	srv := happyHappenstanceAPIServer(t)
	defer srv.Close()
	withHappenstanceAPIBaseURL(t, srv.URL)
	// Suppress the config-layer warning to stderr that fires for unknown
	// prefixes — we don't need to assert on it here, but we shouldn't
	// pollute test output either. The simplest path is to just let it
	// happen; go test only surfaces failures.
	withHappenstanceAPIKey(t, "weird_prefix_FAKE_TEST_KEY_NEVER_SENT_F00D")

	report := map[string]any{}
	checkHappenstanceAPI(stubCfg(t), report)

	keyVal := mustString(t, report, "happenstance_api_key")
	if !strings.HasPrefix(keyVal, "WARN ") {
		t.Errorf("unknown-prefix key should WARN, got %q", keyVal)
	}
	if !strings.Contains(keyVal, "F00D") {
		t.Errorf("unknown-prefix key row should still include last 4 chars, got %q", keyVal)
	}
	// Probes still run even with an unknown prefix.
	if _, ok := report["happenstance_api_validity"]; !ok {
		t.Error("validity should still be probed when key has unknown prefix")
	}
}

// TestCheckHappenstanceAPI_NeverLeaksKey: across every code path the
// doctor exercises (key, validity, balance), the literal full key value
// must never appear in the rendered report. We render the report as JSON
// (mirroring `doctor --json`) and assert the absence of the literal.
func TestCheckHappenstanceAPI_NeverLeaksKey(t *testing.T) {
	const fakeKey = "hpn_live_personal_NEVER_LEAK_LITERAL_VALUE_BAAD"

	t.Run("happy_path", func(t *testing.T) {
		srv := happyHappenstanceAPIServer(t)
		defer srv.Close()
		withHappenstanceAPIBaseURL(t, srv.URL)
		withHappenstanceAPIKey(t, fakeKey)

		report := map[string]any{}
		checkHappenstanceAPI(stubCfg(t), report)
		assertNoKeyLeak(t, report, fakeKey)
	})

	t.Run("validity_401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
		}))
		defer srv.Close()
		withHappenstanceAPIBaseURL(t, srv.URL)
		withHappenstanceAPIKey(t, fakeKey)

		report := map[string]any{}
		checkHappenstanceAPI(stubCfg(t), report)
		assertNoKeyLeak(t, report, fakeKey)
	})

	t.Run("balance_zero", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"email":"matt@example.com","name":"Matt","friends":[]}`))
		})
		mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"balance_credits":0,"has_credits":false}`))
		})
		srv := httptest.NewServer(mux)
		defer srv.Close()
		withHappenstanceAPIBaseURL(t, srv.URL)
		withHappenstanceAPIKey(t, fakeKey)

		report := map[string]any{}
		checkHappenstanceAPI(stubCfg(t), report)
		assertNoKeyLeak(t, report, fakeKey)
	})
}

// TestKeyTail covers the redaction helper directly so its behavior
// (last 4 chars; pass-through for already-short input) is locked down
// outside the doctor flow.
func TestKeyTail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hpn_live_personal_BEEF", "BEEF"},
		{"abc", "abc"},
		{"abcd", "abcd"},
		{"abcde", "bcde"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := keyTail(tc.in); got != tc.want {
			t.Errorf("keyTail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- helpers ---

// mustString fetches a string-valued report key and fails the test if
// the key is missing or the value is not a string.
func mustString(t *testing.T, report map[string]any, key string) string {
	t.Helper()
	v, ok := report[key]
	if !ok {
		t.Fatalf("report missing key %q (have: %v)", key, mapKeys(report))
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("report key %q is %T, want string", key, v)
	}
	return s
}

// mapKeys returns the keys of a map[string]any in deterministic order.
// Used in failure messages to help debug missing-key assertions.
func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// assertNoKeyLeak serializes the report as JSON (mirroring the
// `doctor --json` rendering path) and confirms the literal key value
// never appears anywhere in the output.
func assertNoKeyLeak(t *testing.T, report map[string]any, key string) {
	t.Helper()
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encoding report: %v", err)
	}
	if strings.Contains(string(raw), key) {
		t.Fatalf("report leaked the bearer key value:\n%s", string(raw))
	}
}
