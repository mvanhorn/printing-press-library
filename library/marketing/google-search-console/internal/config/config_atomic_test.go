// PATCH(oauth-login): unit tests for atomic config save and the new
// SaveClient/ForgetAll/HasClient/AccessTokenExpired helpers added for the
// OAuth login flow.

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveTokensAtomicallyAndRoundtrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}

	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)
	if err := cfg.SaveTokens("cid", "csecret", "atoken", "rtoken", expiry); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	// File should exist, mode 0600, no .tmp left behind.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved file: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, want 0600", st.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp leftover: stat returned %v (want IsNotExist)", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ClientID != "cid" || loaded.ClientSecret != "csecret" {
		t.Errorf("client roundtrip: id=%q secret=%q", loaded.ClientID, loaded.ClientSecret)
	}
	if loaded.AccessToken != "atoken" || loaded.RefreshToken != "rtoken" {
		t.Errorf("token roundtrip: access=%q refresh=%q", loaded.AccessToken, loaded.RefreshToken)
	}
	if !loaded.TokenExpiry.Equal(expiry) {
		t.Errorf("expiry roundtrip: got %v want %v", loaded.TokenExpiry, expiry)
	}
}

func TestSaveClientPreservesExistingTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}

	if err := cfg.SaveTokens("oldid", "oldsecret", "atoken", "rtoken", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if err := cfg.SaveClient("newid", "newsecret"); err != nil {
		t.Fatalf("SaveClient: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.ClientID != "newid" || reloaded.ClientSecret != "newsecret" {
		t.Errorf("SaveClient didn't update creds: got id=%q secret=%q", reloaded.ClientID, reloaded.ClientSecret)
	}
	if reloaded.AccessToken != "atoken" || reloaded.RefreshToken != "rtoken" {
		t.Errorf("SaveClient nuked tokens: access=%q refresh=%q", reloaded.AccessToken, reloaded.RefreshToken)
	}
}

func TestForgetAllNukesEverything(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}

	if err := cfg.SaveTokens("cid", "csecret", "atoken", "rtoken", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if err := cfg.ForgetAll(); err != nil {
		t.Fatalf("ForgetAll: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.ClientID != "" || reloaded.ClientSecret != "" || reloaded.AccessToken != "" || reloaded.RefreshToken != "" {
		t.Errorf("ForgetAll left state: %+v", reloaded)
	}
	if !reloaded.TokenExpiry.IsZero() {
		t.Errorf("ForgetAll left token_expiry: %v", reloaded.TokenExpiry)
	}
}

func TestClearTokensKeepsClient(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}

	if err := cfg.SaveTokens("cid", "csecret", "atoken", "rtoken", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.ClientID != "cid" || reloaded.ClientSecret != "csecret" {
		t.Errorf("ClearTokens nuked client: id=%q secret=%q", reloaded.ClientID, reloaded.ClientSecret)
	}
	if reloaded.AccessToken != "" || reloaded.RefreshToken != "" || !reloaded.TokenExpiry.IsZero() {
		t.Errorf("ClearTokens left token state: %+v", reloaded)
	}
}

func TestHasClient(t *testing.T) {
	if (&Config{}).HasClient() {
		t.Errorf("empty config HasClient() = true, want false")
	}
	if !(&Config{ClientID: "x"}).HasClient() {
		t.Errorf("config with ClientID HasClient() = false, want true")
	}
}

func TestEnvVarTakesPrecedenceOverFileToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}
	if err := cfg.SaveTokens("cid", "csecret", "file-token", "rtoken", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	t.Setenv("GSC_ACCESS_TOKEN", "env-token-wins")
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "env-token-wins" {
		t.Errorf("env precedence: got AccessToken=%q want env-token-wins", loaded.AccessToken)
	}
	if loaded.AuthSource != "env:GSC_ACCESS_TOKEN" {
		t.Errorf("env precedence: AuthSource=%q want env:GSC_ACCESS_TOKEN", loaded.AuthSource)
	}
	if loaded.AuthHeader() != "Bearer env-token-wins" {
		t.Errorf("env precedence: AuthHeader=%q want Bearer env-token-wins", loaded.AuthHeader())
	}
}
