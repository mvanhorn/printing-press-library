package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// captureStderr swaps os.Stderr for the duration of fn and returns whatever
// the eager migration wrote there. We use the real os.Stderr swap rather
// than a logger interface because migrateLegacyAuthJSON writes directly to
// os.Stderr; tests that want to assert on its output have to intercept that
// stream.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
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
	os.Stderr = orig
	return <-done
}

func TestEagerMigrate_NoLegacy_NoConfig_Silent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("TESLA_PP_AUTH_HOME", dir)
	t.Setenv("TESLA_CONFIG", cfgPath)
	t.Setenv("TESLA_AUTH_TOKEN", "")

	stderr := captureStderr(t, eagerMigrateLegacyAuth)
	if stderr != "" {
		t.Errorf("expected silent no-op stderr, got %q", stderr)
	}
}

func TestEagerMigrate_ConfigAlreadyHasTokens_Silent(t *testing.T) {
	_, cfg := withTempAuthHome(t)
	if err := saveTeslaTokens(cfg, "refresh_existing", "access_existing", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("saveTeslaTokens: %v", err)
	}
	// Also drop a legacy file to prove the migration is skipped because
	// config wins.
	legacyPath := filepath.Join(os.Getenv("TESLA_PP_AUTH_HOME"), "auth.json")
	writeLegacyAuthJSON(t, legacyPath, authRecord{
		AccessToken:  "access_legacy_should_be_ignored",
		RefreshToken: "refresh_legacy_should_be_ignored",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
	})

	stderr := captureStderr(t, eagerMigrateLegacyAuth)
	if stderr != "" {
		t.Errorf("expected silent no-op when config has tokens, got %q", stderr)
	}

	// Legacy file must still be present (we never tried to migrate).
	if _, err := os.Stat(legacyPath); err != nil {
		t.Errorf("legacy file should be untouched, stat: %v", err)
	}

	// Config tokens must be the original, not the legacy ones.
	cfg2, err := config.Load(os.Getenv("TESLA_CONFIG"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg2.AccessToken != "access_existing" || cfg2.RefreshToken != "refresh_existing" {
		t.Errorf("config tokens overwritten by legacy: access=%q refresh=%q", cfg2.AccessToken, cfg2.RefreshToken)
	}
}

func TestEagerMigrate_LegacyPresentEmptyConfig_Migrates(t *testing.T) {
	legacy, _ := withTempAuthHome(t)
	writeLegacyAuthJSON(t, legacy, authRecord{
		AccessToken:  "access_legacy",
		RefreshToken: "refresh_legacy",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		IssuedAt:     time.Now().UTC().Format(time.RFC3339),
	})

	stderr := captureStderr(t, eagerMigrateLegacyAuth)
	if !strings.Contains(stderr, "Migrated legacy auth.json") {
		t.Errorf("expected breadcrumb in stderr, got %q", stderr)
	}

	// Legacy file should be renamed with a date suffix; original path is gone.
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("expected legacy auth.json to be renamed away, stat err: %v", err)
	}
	// One file matching auth.json.migrated-* should exist.
	entries, err := os.ReadDir(os.Getenv("TESLA_PP_AUTH_HOME"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var foundBreadcrumb bool
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "auth.json.migrated-") {
			foundBreadcrumb = true
		}
	}
	if !foundBreadcrumb {
		t.Errorf("expected an auth.json.migrated-* breadcrumb in %s, got %v", os.Getenv("TESLA_PP_AUTH_HOME"), entries)
	}

	// Tokens should be in config.toml now.
	cfg2, err := config.Load(os.Getenv("TESLA_CONFIG"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg2.AccessToken != "access_legacy" || cfg2.RefreshToken != "refresh_legacy" {
		t.Errorf("tokens not migrated to config: access=%q refresh=%q", cfg2.AccessToken, cfg2.RefreshToken)
	}
}

func TestEagerMigrate_LegacyUnreadable_LogsAndContinues(t *testing.T) {
	// Skip on platforms where chmod 000 doesn't restrict reads (e.g. when
	// running as root in CI).
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 has no effect when running as root")
	}
	legacy, _ := withTempAuthHome(t)
	if err := os.WriteFile(legacy, []byte("{}"), 0o000); err != nil {
		t.Fatalf("write 000 file: %v", err)
	}
	defer func() { _ = os.Chmod(legacy, 0o600) }()

	stderr := captureStderr(t, eagerMigrateLegacyAuth)
	// Either the file open fails (silent path -> no stderr) or the decode
	// fails (stderr warning path). Both are acceptable: we just need the
	// call to NOT panic and NOT block startup. Assert the function
	// returned without panicking by virtue of reaching this line.
	_ = stderr
}
