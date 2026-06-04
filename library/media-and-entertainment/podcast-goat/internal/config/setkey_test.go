// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfig points config Load at a tempdir and restores after.
func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PODCAST_GOAT_CONFIG", filepath.Join(dir, "config.toml"))
	invalidateCache()
	return filepath.Join(dir, "config.toml")
}

func TestSetKey_SpokenPersisted(t *testing.T) {
	path := withTempConfig(t)
	os.Unsetenv("SPOKEN_API_KEY")

	wroteAt, err := SetKey("spoken", "pt_test123")
	if err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if wroteAt != path {
		t.Errorf("wroteAt=%q want %q", wroteAt, path)
	}
	if got := Resolve("SPOKEN_API_KEY"); got != "pt_test123" {
		t.Errorf("Resolve = %q after SetKey, want pt_test123", got)
	}
	if got := Source("SPOKEN_API_KEY"); got != "config" {
		t.Errorf("Source = %q, want config", got)
	}
}

func TestSetKey_EnvWinsOverConfig(t *testing.T) {
	withTempConfig(t)
	t.Setenv("SPOKEN_API_KEY", "pt_from_env")

	_, err := SetKey("spoken", "pt_from_config")
	if err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if got := Resolve("SPOKEN_API_KEY"); got != "pt_from_env" {
		t.Errorf("Resolve = %q, want pt_from_env (env should win)", got)
	}
	if got := Source("SPOKEN_API_KEY"); got != "env" {
		t.Errorf("Source = %q, want env", got)
	}
}

func TestSetKey_UnknownProviderErrors(t *testing.T) {
	withTempConfig(t)
	_, err := SetKey("bogus", "x")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestSetKey_UnsetClearsField(t *testing.T) {
	withTempConfig(t)
	os.Unsetenv("SPOKEN_API_KEY")

	_, _ = SetKey("spoken", "pt_test")
	if got := Resolve("SPOKEN_API_KEY"); got != "pt_test" {
		t.Fatalf("setup: Resolve = %q, want pt_test", got)
	}

	_, err := SetKey("spoken", "")
	if err != nil {
		t.Fatalf("SetKey unset: %v", err)
	}
	if got := Resolve("SPOKEN_API_KEY"); got != "" {
		t.Errorf("Resolve after unset = %q, want empty", got)
	}
	if got := Source("SPOKEN_API_KEY"); got != "missing" {
		t.Errorf("Source after unset = %q, want missing", got)
	}
}

func TestSetKey_AllProvidersRoundTrip(t *testing.T) {
	withTempConfig(t)
	for _, name := range []string{"SPOKEN_API_KEY", "TADDY_API_KEY", "TADDY_USER_ID", "OPENAI_API_KEY", "DEEPGRAM_API_KEY", "ELEVENLABS_API_KEY"} {
		os.Unsetenv(name)
	}
	for _, p := range []string{"spoken", "taddy", "taddy_user_id", "openai", "deepgram", "elevenlabs"} {
		val := "test-" + p
		if _, err := SetKey(p, val); err != nil {
			t.Errorf("SetKey(%s): %v", p, err)
			continue
		}
		got := Resolve(EnvVarFor(p))
		if got != val {
			t.Errorf("provider %s: Resolve = %q, want %q", p, got, val)
		}
	}
}

func TestSetKey_FilePermissions(t *testing.T) {
	path := withTempConfig(t)
	os.Unsetenv("SPOKEN_API_KEY")
	if _, err := SetKey("spoken", "pt_test"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	mode := st.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("config file perms = %o, want 0600 (it's a credentials file)", mode)
	}
}
