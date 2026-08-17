// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/types"
)

const authConfigCredentialSentinel = "synthetic-credential-must-not-escape"

func TestSanitizeAuthConfigResponseUsesFailClosedAllowlist(t *testing.T) {
	input := make(map[string]any)
	typeOfResponse := reflect.TypeOf(types.AuthConfigResponse{})
	for i := 0; i < typeOfResponse.NumField(); i++ {
		field := typeOfResponse.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		switch field.Type.Kind() {
		case reflect.Bool:
			input[name] = true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			input[name] = 42
		case reflect.Float32, reflect.Float64:
			input[name] = 42.5
		default:
			if isSensitiveAuthConfigField(name) {
				input[name] = authConfigCredentialSentinel + "-" + name
			} else {
				input[name] = "approved-" + name
			}
		}
	}
	input["future_unknown_credential"] = authConfigCredentialSentinel + "-future"
	input["site_url"] = "https://portal.example.test"

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	sanitized, err := SanitizeAuthConfigResponse(raw)
	if err != nil {
		t.Fatalf("SanitizeAuthConfigResponse() error = %v", err)
	}
	assertAuthConfigSentinelAbsent(t, sanitized)

	var output map[string]json.RawMessage
	if err := json.Unmarshal(sanitized, &output); err != nil {
		t.Fatalf("sanitized output is invalid JSON: %v", err)
	}
	for i := 0; i < typeOfResponse.NumField(); i++ {
		name := strings.Split(typeOfResponse.Field(i).Tag.Get("json"), ",")[0]
		_, present := output[name]
		if isSensitiveAuthConfigField(name) && present {
			t.Errorf("sensitive field %q was not removed", name)
		}
		if !isSensitiveAuthConfigField(name) && !present {
			t.Errorf("approved field %q was removed", name)
		}
	}
	if _, present := output["future_unknown_credential"]; present {
		t.Error("unknown field escaped the fail-closed allowlist")
	}
}

func TestAuthConfigRequestPreviewRetainsRedactedFieldNames(t *testing.T) {
	preview, err := sanitizeAuthConfigRequestPreview(json.RawMessage(`{
		"external_google_secret":"synthetic-credential-must-not-escape-google",
		"site_url":"https://portal.example.test"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, preview)
	if !bytes.Contains(preview, []byte(`"external_google_secret":"[REDACTED]"`)) {
		t.Fatalf("preview did not retain a redacted field name: %s", preview)
	}
}

func TestAuthConfigFieldClassificationCoversCurrentSchemaExactly(t *testing.T) {
	typeOfResponse := reflect.TypeOf(types.AuthConfigResponse{})
	schemaFields := make(map[string]struct{}, typeOfResponse.NumField())
	for i := 0; i < typeOfResponse.NumField(); i++ {
		name := strings.Split(typeOfResponse.Field(i).Tag.Get("json"), ",")[0]
		schemaFields[name] = struct{}{}
		_, safe := auditedSafeAuthConfigFields[name]
		_, sensitive := auditedSensitiveAuthConfigFields[name]
		if safe == sensitive {
			t.Errorf("auth-config schema field %q must be classified exactly once (safe=%t sensitive=%t)", name, safe, sensitive)
		}
	}
	if got, want := len(auditedSafeAuthConfigFields)+len(auditedSensitiveAuthConfigFields), len(schemaFields); got != want {
		t.Fatalf("audited auth-config field count = %d, schema count = %d", got, want)
	}
	for name := range auditedSafeAuthConfigFields {
		if _, exists := schemaFields[name]; !exists {
			t.Errorf("audited safe field %q is absent from AuthConfigResponse", name)
		}
	}
	for name := range auditedSensitiveAuthConfigFields {
		if _, exists := schemaFields[name]; !exists {
			t.Errorf("audited sensitive field %q is absent from AuthConfigResponse", name)
		}
	}
	for _, name := range []string{
		"rate_limit_otp",
		"rate_limit_token_refresh",
		"refresh_token_rotation_enabled",
		"security_refresh_token_reuse_interval",
		"hook_custom_access_token_enabled",
		"password_min_length",
		"password_required_characters",
		"passkey_enabled",
	} {
		if _, safe := auditedSafeAuthConfigFields[name]; !safe {
			t.Errorf("safe lookalike %q is not explicitly classified safe", name)
		}
	}
}

func TestAuthConfigRequestClassificationCoversCurrentSchema(t *testing.T) {
	typeOfRequest := reflect.TypeOf(types.UpdateAuthConfigBody{})
	for i := 0; i < typeOfRequest.NumField(); i++ {
		name := strings.Split(typeOfRequest.Field(i).Tag.Get("json"), ",")[0]
		_, safe := auditedSafeAuthConfigFields[name]
		_, sensitive := auditedSensitiveAuthConfigFields[name]
		if safe == sensitive {
			t.Errorf("auth-config request field %q must be classified exactly once (safe=%t sensitive=%t)", name, safe, sensitive)
		}
	}
}

func TestSanitizeAuthConfigResponseRejectsNonScalarApprovedValues(t *testing.T) {
	for _, value := range []string{
		`{"site_url":{"sharedKey":"synthetic-credential-must-not-escape"}}`,
		`{"site_url":["synthetic-credential-must-not-escape"]}`,
	} {
		_, err := SanitizeAuthConfigResponse(json.RawMessage(value))
		if err == nil {
			t.Fatalf("non-scalar approved value %s did not fail closed", value)
		}
		assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
	}
}

func TestSanitizeAuthConfigResponseRejectsMalformedJSON(t *testing.T) {
	if _, err := SanitizeAuthConfigResponse(json.RawMessage(`not-json`)); err == nil {
		t.Fatal("malformed protected response did not fail closed")
	}
}

func TestAuthConfigRequestRejectsUnknownFieldsInLiveAndDryRun(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write(authConfigFixture())
	}))
	defer server.Close()

	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dry-run=%t", dryRun), func(t *testing.T) {
			c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
			c.DryRun = dryRun
			_, _, err := c.Patch("/v1/projects/project-ref/config/auth", map[string]any{
				"future_unknown_credential": authConfigCredentialSentinel,
			})
			if err == nil || !strings.Contains(err.Error(), "future_unknown_credential") {
				t.Fatalf("unknown request field did not fail closed: %v", err)
			}
			assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("unknown request reached the network %d time(s)", got)
	}
}

func TestSanitizeAuthConfigResponseDropsCredentialBearingHookURI(t *testing.T) {
	raw := json.RawMessage(`{
		"site_url":"https://portal.example.test",
		"hook_send_email_uri":"https://user:synthetic-credential-must-not-escape@hooks.example.test/send?token=synthetic-credential-must-not-escape"
	}`)
	sanitized, err := SanitizeAuthConfigResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, sanitized)
	var output map[string]json.RawMessage
	if err := json.Unmarshal(sanitized, &output); err != nil {
		t.Fatal(err)
	}
	if _, present := output["hook_send_email_uri"]; present {
		t.Fatal("credential-capable Auth Hook URI escaped the denylist")
	}
}

func TestAuthConfigCachePurgeFailureStopsProtectedRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(authConfigFixture())
	}))
	defer server.Close()

	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
			c.cacheDir = t.TempDir()
			c.removeAll = func(string) error { return errors.New("synthetic purge failure") }
			before := requests
			var err error
			if method == http.MethodGet {
				_, err = c.Get("/v1/projects/project-ref/config/auth", nil)
			} else {
				_, _, err = c.Patch("/v1/projects/project-ref/config/auth", map[string]any{"site_url": "https://portal.example.test"})
			}
			if err == nil || !strings.Contains(err.Error(), "purging legacy auth-config cache") {
				t.Fatalf("protected %s did not fail closed on cache purge error: %v", method, err)
			}
			assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
			if requests != before {
				t.Fatalf("protected %s reached the network after cache purge failure", method)
			}
		})
	}
}

func TestAuthConfigGetPurgesLegacyCacheAndNeverCachesResponse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cacheDir := filepath.Join(home, ".cache", "supabase-pp-cli", "http")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			t.Errorf("legacy cache still existed when protected GET was sent: %v", err)
		}
		_, _ = w.Write(authConfigFixture())
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(c.cacheDir, "legacy-auth-config.json")
	if err := os.WriteFile(legacyPath, authConfigFixture(), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-time.Hour)
	if err := os.Chtimes(legacyPath, stale, stale); err != nil {
		t.Fatal(err)
	}

	path := "/v1/projects/project-ref/config/auth?include=all"
	response, err := c.Get(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, response)
	if _, err := os.Stat(c.cacheDir); !os.IsNotExist(err) {
		t.Fatalf("protected GET recreated an HTTP cache directory: %v", err)
	}
}

func TestAuthConfigPatchPurgesLegacyCacheBeforeRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cacheDir := filepath.Join(home, ".cache", "supabase-pp-cli", "http")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
			t.Errorf("legacy cache still existed when protected PATCH was sent: %v", err)
		}
		_, _ = w.Write(authConfigFixture())
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.cacheDir, "legacy-auth-config.json"), authConfigFixture(), 0o644); err != nil {
		t.Fatal(err)
	}

	response, _, err := c.Patch("/v1/projects/project-ref/config/auth?mode=rotate", map[string]any{
		"external_google_secret": authConfigCredentialSentinel + "-request",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, response)
	if _, err := os.Stat(c.cacheDir); !os.IsNotExist(err) {
		t.Fatalf("protected PATCH left an HTTP cache directory: %v", err)
	}
}

func TestAuthConfigPatchSanitizesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		_, _ = w.Write(authConfigFixture())
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	response, _, err := c.Patch("/v1/projects/project-ref/config/auth?mode=rotate", map[string]any{
		"external_google_secret": authConfigCredentialSentinel + "-request",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, response)
}

func TestAuthConfigErrorDropsResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(authConfigCredentialSentinel))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	_, err := c.Get("/v1/projects/project-ref/config/auth", nil)
	if err == nil {
		t.Fatal("Get() returned nil error")
	}
	assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
	if !strings.Contains(err.Error(), "response body redacted") {
		t.Fatalf("sanitized error omitted redaction receipt: %v", err)
	}
}

func TestAuthConfigDryRunRedactsRequestBody(t *testing.T) {
	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = reader.Close()
		_ = writer.Close()
	})

	c := New(&config.Config{BaseURL: "https://api.example.test", AccessToken: "synthetic-token"}, time.Second, 0)
	c.DryRun = true
	response, _, err := c.Patch("/v1/projects/project-ref/config/auth?mode=rotate", map[string]any{
		"external_google_secret": authConfigCredentialSentinel + "-request",
		"site_url":               "https://portal.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = oldStderr
	stderr, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	assertAuthConfigSentinelAbsent(t, stderr)
	assertAuthConfigSentinelAbsent(t, response)
	if bytes.Contains(stderr, []byte("****oken")) {
		t.Fatalf("dry run exposed an authorization-token suffix: %s", stderr)
	}
	if !bytes.Contains(stderr, []byte("Authorization: [REDACTED]")) {
		t.Fatalf("dry run omitted full authorization redaction: %s", stderr)
	}
	if !bytes.Contains(stderr, []byte("portal.example.test")) {
		t.Fatalf("dry run omitted approved field: %s", stderr)
	}
}

func TestIsAuthConfigPathCanonicalizesProtectedEndpointVariants(t *testing.T) {
	for _, requestPath := range []string{
		"/v1/projects/project-ref/config/auth",
		"/v1/projects/project-ref/config/auth/",
		"/v1/projects/project-ref/config/auth?include=all",
		"https://api.supabase.com/v1/projects/project-ref/config/auth?include=all",
		"//v1//projects//project-ref//config//auth",
		"/v1%2fprojects%2fproject-ref%2fconfig%2fauth",
		"/v1/projects/project-ref/config%2Fauth",
		"/v1/projects/ignored/../project-ref/config/auth",
		"/v1/projects/ignored/%2e%2e/project-ref/config/auth",
		`\v1\projects\project-ref\config\auth`,
	} {
		if !IsAuthConfigPath(requestPath) {
			t.Errorf("protected auth-config path variant %q was not recognized", requestPath)
		}
	}
}

func TestIsAuthConfigPathExcludesNestedAuthEndpoints(t *testing.T) {
	for _, requestPath := range []string{
		"/v1/projects/project-ref/config/auth/sso/providers",
		"/v1/projects/project-ref/config/auth/signing-keys",
		"/v1/projects/project-ref/config/database",
		"/v1/projects/project-ref/config/database?next=/config/auth",
	} {
		if IsAuthConfigPath(requestPath) {
			t.Errorf("nested or unrelated path %q was misclassified", requestPath)
		}
	}
}

func TestAuthConfigCanonicalizationRedirectFailsClosed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/projects/project-ref/config/auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(authConfigFixture())
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	_, err := c.Get("/v1/projects/ignored/../project-ref/config/auth", nil)
	if err == nil || !strings.Contains(err.Error(), "redirect into protected auth-config endpoint refused") {
		t.Fatalf("canonicalization redirect did not fail closed: %v", err)
	}
	assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
}

func TestUnprotectedGetCannotRedirectIntoAuthConfig(t *testing.T) {
	protectedReached := false
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v1/projects/project-ref/config/auth?token="+authConfigCredentialSentinel, http.StatusFound)
	})
	mux.HandleFunc("/v1/projects/project-ref/config/auth", func(w http.ResponseWriter, r *http.Request) {
		protectedReached = true
		_, _ = w.Write(authConfigFixture())
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	c.cacheDir = filepath.Join(t.TempDir(), "http-cache")
	_, err := c.Get("/redirect", nil)
	if err == nil || !strings.Contains(err.Error(), "redirect into protected auth-config endpoint refused") {
		t.Fatalf("redirect into protected auth config did not fail closed: %v", err)
	}
	assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
	if protectedReached {
		t.Fatal("redirect reached the credential-bearing auth-config endpoint")
	}
	if _, err := os.Stat(c.cacheDir); !os.IsNotExist(err) {
		t.Fatalf("rejected redirect created a cache directory: %v", err)
	}
}

func TestAuthConfigRequestRefusesEveryRedirectWithoutLeakingDestination(t *testing.T) {
	var targetReached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReached.Store(true)
		if r.Header.Get("Authorization") != "" {
			t.Error("protected Authorization header reached redirect target")
		}
		_, _ = w.Write(authConfigFixture())
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture?token="+authConfigCredentialSentinel, http.StatusFound)
	}))
	defer origin.Close()

	c := New(&config.Config{BaseURL: origin.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	_, err := c.Get("/v1/projects/project-ref/config/auth", nil)
	if err == nil || !strings.Contains(err.Error(), "redirect into protected auth-config endpoint refused") {
		t.Fatalf("protected auth-config redirect did not fail closed: %v", err)
	}
	assertAuthConfigSentinelAbsent(t, []byte(err.Error()))
	if targetReached.Load() {
		t.Fatal("protected auth-config redirect reached its destination")
	}
}

func TestNonAuthConfigResponseRemainsByteEquivalent(t *testing.T) {
	original := []byte(`{"external_google_secret":"not-protected-on-unrelated-path"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(original)
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL, AccessToken: "synthetic-token"}, time.Second, 0)
	c.NoCache = true
	response, err := c.Get("/v1/projects/project-ref/config/database", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(response, original) {
		t.Fatalf("unrelated response changed: got %s, want %s", response, original)
	}
}

func authConfigFixture() []byte {
	return []byte(`{
		"site_url":"https://portal.example.test",
		"external_google_enabled":true,
		"external_google_secret":"synthetic-credential-must-not-escape-google",
		"smtp_pass":"synthetic-credential-must-not-escape-smtp",
		"future_unknown_credential":"synthetic-credential-must-not-escape-future"
	}`)
}

func assertAuthConfigSentinelAbsent(t *testing.T, data []byte) {
	t.Helper()
	if bytes.Contains(data, []byte(authConfigCredentialSentinel)) {
		t.Fatalf("credential sentinel escaped: %s", data)
	}
}
