// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// captureStderr runs fn while redirecting os.Stderr to a pipe, returning
// whatever fn wrote. Used for the prefix-warning scenarios.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = orig
	})

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	return <-done
}

// writeConfig writes TOML content to a temp file and returns the path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// Happy: env var set, no config file -> returns env value.
func TestLoadAPIKey_EnvOnly_NoConfigFile(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "hpn_live_personal_envvalue123")

	cfg := &Config{Path: filepath.Join(t.TempDir(), "does-not-exist.toml")}

	got := LoadAPIKey(cfg)
	if got != "hpn_live_personal_envvalue123" {
		t.Fatalf("LoadAPIKey returned %q, want env value", got)
	}
}

// Happy: env var unset, config file has happenstance_api_key -> returns config value.
func TestLoadAPIKey_ConfigOnly(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "")

	path := writeConfig(t, `happenstance_api_key = "hpn_live_personal_fromconfig"`)
	cfg := &Config{Path: path}

	got := LoadAPIKey(cfg)
	if got != "hpn_live_personal_fromconfig" {
		t.Fatalf("LoadAPIKey returned %q, want config value", got)
	}
}

// Edge: env set AND config set with different values -> env wins.
func TestLoadAPIKey_EnvWinsOverConfig(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "hpn_live_personal_env")

	path := writeConfig(t, `happenstance_api_key = "hpn_live_personal_config"`)
	cfg := &Config{Path: path}

	got := LoadAPIKey(cfg)
	if got != "hpn_live_personal_env" {
		t.Fatalf("LoadAPIKey returned %q, want env value (env should win over config)", got)
	}
}

// Edge: env set with value that does not start with hpn_live_ -> warning emitted, key still returned.
func TestLoadAPIKey_UnknownPrefix_WarnsAndReturns(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "garbage_prefix_xyz")
	cfg := &Config{Path: filepath.Join(t.TempDir(), "absent.toml")}

	var got string
	stderr := captureStderr(t, func() {
		got = LoadAPIKey(cfg)
	})

	if got != "garbage_prefix_xyz" {
		t.Fatalf("LoadAPIKey returned %q, want the unmodified key (warning is informational only)", got)
	}
	if !strings.Contains(stderr, "warning") {
		t.Fatalf("expected warning on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "HAPPENSTANCE_API_KEY") {
		t.Fatalf("expected env-var name in warning, got: %q", stderr)
	}
}

// Confirm a recognized prefix produces no warning.
func TestLoadAPIKey_KnownPrefix_NoWarning(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "hpn_live_personal_legitimatekey")
	cfg := &Config{Path: filepath.Join(t.TempDir(), "absent.toml")}

	var got string
	stderr := captureStderr(t, func() {
		got = LoadAPIKey(cfg)
	})

	if got != "hpn_live_personal_legitimatekey" {
		t.Fatalf("LoadAPIKey returned %q, want the env value", got)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for recognized prefix, got: %q", stderr)
	}
}

// Edge: both unset -> empty string.
func TestLoadAPIKey_BothUnset_ReturnsEmpty(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "")
	cfg := &Config{Path: filepath.Join(t.TempDir(), "no-such-file.toml")}

	got := LoadAPIKey(cfg)
	if got != "" {
		t.Fatalf("LoadAPIKey returned %q, want empty string", got)
	}
}

// Edge: nil cfg + no env -> empty string (must not panic).
func TestLoadAPIKey_NilConfig_NoEnv_Empty(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "")
	got := LoadAPIKey(nil)
	if got != "" {
		t.Fatalf("LoadAPIKey(nil) returned %q, want empty", got)
	}
}

// Edge: nil cfg + env set -> returns env value (must not panic on cfg.Path).
func TestLoadAPIKey_NilConfig_WithEnv(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "hpn_live_personal_envonly")
	got := LoadAPIKey(nil)
	if got != "hpn_live_personal_envonly" {
		t.Fatalf("LoadAPIKey(nil) with env returned %q, want env value", got)
	}
}

// Edge: TOML round-trip preserves the field's exact value.
//
// Strategy: write a config with a known key, decode via HappenstanceAPI,
// re-encode, decode again, confirm the value survived unchanged. We also
// confirm the field name in the re-encoded output matches the spec
// (`happenstance_api_key`).
func TestLoadAPIKey_TOMLRoundTrip(t *testing.T) {
	const original = "hpn_live_personal_roundtrip_value_with_underscores_and_digits_42"

	path := writeConfig(t, `happenstance_api_key = "`+original+`"`)
	cfg := &Config{Path: path}

	// First load via the public accessor (no env so the file is consulted).
	t.Setenv(HappenstanceAPIKeyEnvVar, "")
	got := LoadAPIKey(cfg)
	if got != original {
		t.Fatalf("first load: LoadAPIKey returned %q, want %q", got, original)
	}

	// Round-trip: re-marshal a HappenstanceAPI containing the value, write it,
	// re-load, confirm bit-for-bit identity.
	hp := HappenstanceAPI{APIKey: got}
	encoded, err := toml.Marshal(hp)
	if err != nil {
		t.Fatalf("toml.Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), "happenstance_api_key") {
		t.Fatalf("re-encoded TOML missing field name, got: %q", string(encoded))
	}
	if !strings.Contains(string(encoded), original) {
		t.Fatalf("re-encoded TOML missing field value, got: %q", string(encoded))
	}

	roundTripPath := filepath.Join(t.TempDir(), "roundtrip.toml")
	if err := os.WriteFile(roundTripPath, encoded, 0o600); err != nil {
		t.Fatalf("write round-trip file: %v", err)
	}
	cfg2 := &Config{Path: roundTripPath}
	got2 := LoadAPIKey(cfg2)
	if got2 != original {
		t.Fatalf("after round-trip: LoadAPIKey returned %q, want %q", got2, original)
	}
}

// Confirm that loading the config file twice (once into Config, once into
// HappenstanceAPI) does not corrupt either decode. This is the "two-pass"
// invariant the sibling-file approach depends on.
func TestLoadAPIKey_DualDecode_NoCrosstalk(t *testing.T) {
	t.Setenv(HappenstanceAPIKeyEnvVar, "")
	t.Setenv("HAPPENSTANCE_WEB_APP_COOKIE_AUTH", "")

	const apiKey = "hpn_live_personal_dualdecode"
	const cookieAuth = "Bearer cookie-token-from-web-app"
	const baseURL = "https://example.test"

	tomlBody := "" +
		`base_url = "` + baseURL + `"` + "\n" +
		`web_app_cookie_auth = "` + cookieAuth + `"` + "\n" +
		`happenstance_api_key = "` + apiKey + `"` + "\n"

	path := writeConfig(t, tomlBody)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != baseURL {
		t.Errorf("Config.BaseURL = %q, want %q", cfg.BaseURL, baseURL)
	}
	if cfg.HappenstanceWebAppCookieAuth != cookieAuth {
		t.Errorf("Config.HappenstanceWebAppCookieAuth = %q, want %q", cfg.HappenstanceWebAppCookieAuth, cookieAuth)
	}

	got := LoadAPIKey(cfg)
	if got != apiKey {
		t.Errorf("LoadAPIKey = %q, want %q (sibling decode must not be disturbed by Config decode)", got, apiKey)
	}
}
