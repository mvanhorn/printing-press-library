// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

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

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/config"
)

func TestAuthSetBearerTokenStoresAppOnlyBearerField(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("access_token = \"old-ambiguous-token\"\noauth2_user_token = \"user-token\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "set-bearer-token", "bearer-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-bearer-token failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["auth_lane"] != "app_only_bearer" || payload["saved"] != true {
		t.Fatalf("payload = %#v", payload)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.XBearerToken != "bearer-token" {
		t.Fatalf("bearer token not stored in bearer_token: %+v", cfg)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("ambiguous access_token should be cleared, got %q", cfg.AccessToken)
	}
	if cfg.XOauth2UserToken != "user-token" {
		t.Fatalf("user-context token should be preserved, got %q", cfg.XOauth2UserToken)
	}
	if got := cfg.AppOnlyAuthHeader(); got != "Bearer bearer-token" {
		t.Fatalf("AppOnlyAuthHeader() = %q", got)
	}
}

func TestAuthSetTokenDeprecatedAliasStoresBearerField(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "set-token", "bearer-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set-token alias failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	msg, ok := payload["deprecated_alias"].(string)
	if payload["auth_lane"] != "app_only_bearer" || !ok || msg == "" {
		t.Fatalf("payload should identify app-only lane and deprecation: %#v", payload)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.XBearerToken != "bearer-token" || cfg.AccessToken != "" {
		t.Fatalf("set-token alias should write bearer_token, not access_token: %+v", cfg)
	}
}

func TestAuthSetBearerTokenRejectsEmptyToken(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "set-bearer-token", "   ",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected whitespace-only bearer token to fail")
	}
	if !strings.Contains(err.Error(), "bearer token must not be empty") {
		t.Fatalf("expected empty-token error, got %v", err)
	}
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("empty token should not write config file, stat err = %v", statErr)
	}
}

func TestAuthSetupAndStatusHelpIncludeClientIDForOAuth2Import(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{name: "setup", args: []string{"--config", configPath, "auth", "setup"}},
		{name: "status unauthenticated", args: []string{"--config", configPath, "auth", "status"}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var flags rootFlags
			cmd := newRootCmd(&flags)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected auth status to fail without credentials")
				}
			} else if err != nil {
				t.Fatalf("command failed: %v\noutput: %s", err, out.String())
			}
			output := out.String()
			if !strings.Contains(output, "auth import-oauth2 --client-id <oauth2-client-id> --access-token") {
				t.Fatalf("OAuth2 import guidance should include --client-id, output:\n%s", output)
			}
			if strings.Contains(output, "auth import-oauth2 --access-token") {
				t.Fatalf("OAuth2 import guidance still shows stale command without --client-id, output:\n%s", output)
			}
		})
	}
}

func TestAuthImportOAuth2StoresUserContextMetadata(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("oauth2_user_token = \"old-token\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--client-id", "client-id",
		"--refresh-token", "refresh-token",
		"--scopes", "tweet.read,tweet.write,users.read,bookmark.read,dm.read,dm.write,follows.read,like.read,offline.access",
		"--expires-at", "2026-06-08T12:00:00Z",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["auth_lane"] != "oauth2_user_context" || payload["refresh_token_present"] != true || payload["client_id_present"] != true {
		t.Fatalf("payload = %#v", payload)
	}
	if _, ok := payload["env_override_warning"]; ok {
		t.Fatalf("unexpected env_override_warning: %#v", payload)
	}
	if _, ok := payload["missing_for"]; ok {
		t.Fatalf("unexpected missing_for with complete imported scopes: %#v", payload)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.XOauth2UserToken != "user-token" || cfg.AccessToken != "" || cfg.RefreshToken != "refresh-token" || cfg.ClientID != "client-id" {
		t.Fatalf("tokens not stored in user-context fields: %+v", cfg)
	}
	if cfg.UserContextAuthHeader() != "Bearer user-token" {
		t.Fatalf("imported token is shadowed: oauth2_user_token=%q header=%q", cfg.XOauth2UserToken, cfg.UserContextAuthHeader())
	}
	if len(cfg.Scopes) != 9 || cfg.Scopes[0] != "tweet.read" {
		t.Fatalf("scopes = %#v", cfg.Scopes)
	}
	if cfg.TokenExpiry.UTC().Format("2006-01-02T15:04:05Z") != "2026-06-08T12:00:00Z" {
		t.Fatalf("expiry = %s", cfg.TokenExpiry)
	}
}

func TestAuthImportOAuth2WarnsWhenEnvTokenWouldShadowImport(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "env-token")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "imported-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	warning, ok := payload["env_override_warning"].(string)
	if !ok || warning == "" {
		t.Fatalf("missing env_override_warning: %#v", payload)
	}
	if !strings.Contains(warning, "X_OAUTH2_USER_TOKEN") || !strings.Contains(warning, "shadow") {
		t.Fatalf("warning does not explain shadowing: %q", warning)
	}
}

func TestAuthImportOAuth2RuntimeHeaderBeatsBearerEnv(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "app-only-token")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "imported-user-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer imported-user-token" {
		t.Fatalf("AuthHeader() should prefer imported user-context token over app-only env token, got %q", got)
	}
	if source := cfg.UserContextAuthSource(); source != "config:oauth2_user_token" {
		t.Fatalf("UserContextAuthSource() = %q", source)
	}
}

func TestAuthImportOAuth2ReportsMissingScopeWorkflowsOnlyWhenPresent(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--scopes", "tweet.read",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	missing, ok := payload["missing_for"].(map[string]any)
	if !ok {
		t.Fatalf("missing_for should be present for incomplete scopes: %#v", payload)
	}
	if _, ok := missing["public_writes"]; !ok {
		t.Fatalf("missing_for should include public_writes: %#v", missing)
	}
}

func TestAuthImportOAuth2RequiresClientIDWithRefreshToken(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--refresh-token", "refresh-token",
		"--json",
	})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--client-id is required") {
		t.Fatalf("expected client-id usage error, got %v\noutput: %s", err, out.String())
	}
}

func TestAuthImportOAuth2PreservesStoredClientID(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("client_id = \"existing-client\"\nclient_secret = \"existing-secret\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", configPath,
		"auth", "import-oauth2",
		"--access-token", "user-token",
		"--refresh-token", "refresh-token",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth import-oauth2 failed: %v\noutput: %s", err, out.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ClientID != "existing-client" || cfg.ClientSecret != "existing-secret" {
		t.Fatalf("client credentials not preserved: %+v", cfg)
	}
}

func TestAuthRefreshPersistsRotatedOAuth2Token(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("oauth2_user_token = \"old-access\"\nrefresh_token = \"old-refresh\"\nclient_id = \"client-id\"\ntoken_expiry = 2026-06-08T12:00:00Z\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2/oauth2/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "old-refresh" || r.Form.Get("client_id") != "client-id" {
			t.Fatalf("unexpected refresh form: %#v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"scope":"tweet.read users.read offline.access"}`))
	}))
	defer server.Close()
	oldEndpoint := client.OAuth2TokenEndpoint
	client.OAuth2TokenEndpoint = server.URL + "/2/oauth2/token"
	defer func() { client.OAuth2TokenEndpoint = oldEndpoint }()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "auth", "refresh", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth refresh failed: %v\noutput: %s", err, out.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload["auth_lane"] != "oauth2_user_context" || payload["refreshed"] != true || payload["refresh_token_rotated"] != true {
		t.Fatalf("payload = %#v", payload)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.XOauth2UserToken != "new-access" || cfg.RefreshToken != "new-refresh" || cfg.AccessToken != "" {
		t.Fatalf("refreshed token tuple not persisted: %+v", cfg)
	}
	if cfg.TokenExpiry.IsZero() || time.Until(cfg.TokenExpiry) < 30*time.Minute {
		t.Fatalf("TokenExpiry = %s, want refreshed future expiry", cfg.TokenExpiry)
	}
}

func TestAuthRefreshReportsRejectedRefreshToken(t *testing.T) {
	t.Setenv("X_BEARER_TOKEN", "")
	t.Setenv("X_OAUTH2_USER_TOKEN", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("oauth2_user_token = \"old-access\"\nrefresh_token = \"dead-refresh\"\nclient_id = \"client-id\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token expired"}`))
	}))
	defer server.Close()
	oldEndpoint := client.OAuth2TokenEndpoint
	client.OAuth2TokenEndpoint = server.URL + "/2/oauth2/token"
	defer func() { client.OAuth2TokenEndpoint = oldEndpoint }()

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "auth", "refresh", "--json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected auth refresh to fail\noutput: %s", out.String())
	}
	if !strings.Contains(err.Error(), "OAuth2 user-context refresh failed") || !strings.Contains(err.Error(), "oauth2-login") {
		t.Fatalf("refresh error should explain re-login path, got: %v", err)
	}
	if strings.Contains(err.Error(), "dead-refresh") {
		t.Fatalf("refresh error leaked refresh token: %v", err)
	}
}
