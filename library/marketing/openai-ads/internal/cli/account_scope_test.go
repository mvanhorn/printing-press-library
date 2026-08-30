package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/cliutil/testenv"
)

func TestScopedDBFilename_usesAccessToken(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("OPENAI_ADS_API_KEY", "")
	t.Setenv("OPENAI_ADS_CONVERSIONS_API_KEY", "")
	t.Cleanup(func() { setActiveConfigPath("") })

	if got := scopedDBFilename(); got != "data.db" {
		t.Fatalf("no credential: got %q, want data.db", got)
	}

	dir := t.TempDir()
	a := filepath.Join(dir, "a.toml")
	b := filepath.Join(dir, "b.toml")
	if err := os.WriteFile(a, []byte("access_token = \"tok-account-a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("access_token = \"tok-account-b\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	setActiveConfigPath(a)
	nameA := scopedDBFilename()
	setActiveConfigPath(b)
	nameB := scopedDBFilename()

	if nameA == "data.db" || nameB == "data.db" {
		t.Fatalf("stored AccessToken still selected the shared database: a=%q b=%q", nameA, nameB)
	}
	if nameA == nameB {
		t.Fatalf("two AccessTokens selected the same database %q", nameA)
	}
}

func TestScopedDBFilename_envKeyWinsOverAccessToken(t *testing.T) {
	testenv.Isolate(t)
	t.Cleanup(func() { setActiveConfigPath("") })

	dir := t.TempDir()
	cfg := filepath.Join(dir, "cfg.toml")
	if err := os.WriteFile(cfg, []byte("access_token = \"tok-account-a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	setActiveConfigPath(cfg)
	t.Setenv("OPENAI_ADS_API_KEY", "")
	tokenName := scopedDBFilename()

	t.Setenv("OPENAI_ADS_API_KEY", "sk-env-account-c")
	envName := scopedDBFilename()
	if envName == "data.db" || envName == tokenName {
		t.Fatalf("env API key should isolate from stored token: env=%q token=%q", envName, tokenName)
	}
}
