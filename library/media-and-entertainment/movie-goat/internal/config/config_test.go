// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing fixture config: %v", err)
	}
	return path
}

func TestOmdbKeyFromConfigFile(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "")
	path := writeConfig(t, "api_key = 'tmdb-placeholder'\nomdb_api_key = 'omdb-placeholder'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.OmdbKey(); got != "omdb-placeholder" {
		t.Errorf("OmdbKey() = %q, want the value stored in config.toml", got)
	}
	if got := cfg.OmdbSource(); got != "config:omdb_api_key" {
		t.Errorf("OmdbSource() = %q, want config:omdb_api_key", got)
	}
}

func TestOmdbEnvTakesPrecedenceOverConfigFile(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "omdb-from-env")
	path := writeConfig(t, "omdb_api_key = 'omdb-from-file'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.OmdbKey(); got != "omdb-from-env" {
		t.Errorf("OmdbKey() = %q, want the environment value to win", got)
	}
	if got := cfg.OmdbSource(); got != "env:OMDB_API_KEY" {
		t.Errorf("OmdbSource() = %q, want env:OMDB_API_KEY", got)
	}
}

func TestOmdbKeyEmptyWhenUnset(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "")
	path := writeConfig(t, "api_key = 'tmdb-placeholder'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.OmdbKey(); got != "" {
		t.Errorf("OmdbKey() = %q, want empty so enrichment degrades gracefully", got)
	}
	if got := cfg.OmdbSource(); got != "" {
		t.Errorf("OmdbSource() = %q, want empty", got)
	}
}

// An OMDb key supplied through the environment must never be persisted by an
// unrelated save. Without the unexported env field, Load() would copy the
// environment value onto the marshaled struct and the next SaveCredential call
// would write that secret into config.toml behind the user's back.
func TestOmdbEnvNeverWrittenToConfigFile(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "omdb-from-env")
	path := writeConfig(t, "api_key = 'old-tmdb-placeholder'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.SaveCredential("new-tmdb-placeholder"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config back: %v", err)
	}
	if strings.Contains(string(data), "omdb-from-env") {
		t.Errorf("config.toml persisted the environment-supplied OMDb key:\n%s", data)
	}
}

func TestClearCredentialsClearsBothInOneWrite(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "")
	t.Setenv("TMDB_API_KEY", "")
	path := writeConfig(t, "api_key = 'tmdb-placeholder'\nomdb_api_key = 'omdb-placeholder'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials: %v", err)
	}

	cleared, err := Load(path)
	if err != nil {
		t.Fatalf("Load after clear: %v", err)
	}
	if got := cleared.TmdbApiKey; got != "" {
		t.Errorf("TmdbApiKey after ClearCredentials = %q, want empty", got)
	}
	if got := cleared.OmdbKey(); got != "" {
		t.Errorf("OmdbKey() after ClearCredentials = %q, want empty", got)
	}
}

func TestSaveAndClearOmdbCredential(t *testing.T) {
	t.Setenv("OMDB_API_KEY", "")
	path := writeConfig(t, "api_key = 'tmdb-placeholder'\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.SaveOmdbCredential("omdb-placeholder"); err != nil {
		t.Fatalf("SaveOmdbCredential: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if got := reloaded.OmdbKey(); got != "omdb-placeholder" {
		t.Errorf("OmdbKey() after save = %q, want the saved value", got)
	}
	if got := reloaded.TmdbApiKey; got != "tmdb-placeholder" {
		t.Errorf("SaveOmdbCredential clobbered the TMDb key: got %q", got)
	}

	if err := reloaded.ClearOmdbCredential(); err != nil {
		t.Fatalf("ClearOmdbCredential: %v", err)
	}
	cleared, err := Load(path)
	if err != nil {
		t.Fatalf("Load after clear: %v", err)
	}
	if got := cleared.OmdbKey(); got != "" {
		t.Errorf("OmdbKey() after clear = %q, want empty", got)
	}
}
