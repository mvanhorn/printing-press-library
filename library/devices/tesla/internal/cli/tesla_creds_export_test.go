// Tests for `tesla auth export`. See plan 2026-05-22-001 U2c.
//
// Style note: every test that exercises the export path swaps out the
// package-level readPassphraseFn so the suite never blocks on stdin. The
// swap is done via withPassphrase below; do NOT call term.ReadPassword
// from tests.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const testCredsPassphrase = "correct horse battery staple 4242"

// withPassphrase substitutes readPassphraseFn for the duration of the test.
// Idempotent and parallel-unsafe (we never call t.Parallel in this suite).
func withPassphrase(t *testing.T, pw string) {
	t.Helper()
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		return []byte(pw), nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })
}

// setupFullyConfiguredCfg writes a config.toml + a real private key file in a
// tempdir and returns the cfg path. Used as the "source machine" state for
// most tests.
func setupFullyConfiguredCfg(t *testing.T) (cfgPath, keyPath string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfgPath = filepath.Join(cfgDir, "config.toml")

	teslaDir := filepath.Join(home, ".tesla")
	if err := os.MkdirAll(teslaDir, 0o700); err != nil {
		t.Fatalf("mkdir tesla: %v", err)
	}
	keyPath = filepath.Join(teslaDir, "test-private.pem")
	if err := os.WriteFile(keyPath, []byte("-----BEGIN EC PRIVATE KEY-----\nFAKEKEYBYTES_TESLA_PP_TEST_SENTINEL\n-----END EC PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load cfg: %v", err)
	}
	// Seed fleet block + iOS-app fields.
	expiry := time.Now().Add(6 * time.Hour).UTC().Truncate(time.Second)
	if err := cfg.SaveFleetTokens(
		"fleet-client-id-1",
		"fleet-client-secret-1",
		"fleet-access-1",
		"fleet-refresh-1",
		expiry,
		"keys.example.com",
		keyPath,
	); err != nil {
		t.Fatalf("save fleet: %v", err)
	}
	if err := cfg.SaveTokens("", "", "ios-access-1", "ios-refresh-1", expiry); err != nil {
		t.Fatalf("save tokens: %v", err)
	}
	return cfgPath, keyPath
}

// runExport invokes the export command with the given args, capturing stdout.
// Returns the exit-style error (cliError wrapping) directly so callers can
// inspect ExitCode.
func runExport(t *testing.T, cfgPath string, args []string) (string, error) {
	t.Helper()
	flags := &rootFlags{configPath: cfgPath, asJSON: true}
	cmd := newCredsExportCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCredsExport_HappyPathAllIncludes(t *testing.T) {
	cfgPath, keyPath := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "bundle.tgz.enc")

	stdout, err := runExport(t, cfgPath, []string{
		"--out", outPath,
		"--include", "keys,fleet,owner-api",
	})
	if err != nil {
		t.Fatalf("export: %v (stdout=%s)", err, stdout)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat bundle: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("bundle mode = %o, want 0600", info.Mode().Perm())
	}

	// Sanity: read raw bytes and assert the magic header is present.
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte(credsBundleMagic)) {
		t.Errorf("bundle missing magic header")
	}

	// Keep key path referenced so the test communicates the source state
	// even when no further assertion fires against it.
	if keyPath == "" {
		t.Fatal("key path unset")
	}
}

func TestCredsExport_DefaultInclude_KeysFleet(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	if _, err := runExport(t, cfgPath, []string{"--out", outPath}); err != nil {
		t.Fatalf("export: %v", err)
	}
	// Decrypt to verify there's no owner-api section when --include is default.
	contents := decryptForTest(t, outPath, testCredsPassphrase)
	if contents.FleetTOML == nil {
		t.Errorf("expected fleet block in default-include bundle")
	}
	if contents.PrivateKey == nil {
		t.Errorf("expected private key in default-include bundle")
	}
	if contents.OwnerAPITOML != nil {
		t.Errorf("did not expect owner-api block in default-include bundle")
	}
}

func TestCredsExport_PartialInclude_FleetOnly(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	if _, err := runExport(t, cfgPath, []string{"--out", outPath, "--include", "fleet"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	contents := decryptForTest(t, outPath, testCredsPassphrase)
	if contents.FleetTOML == nil {
		t.Errorf("expected fleet block")
	}
	if contents.PrivateKey != nil {
		t.Errorf("did not expect private key in --include fleet bundle")
	}
	if contents.OwnerAPITOML != nil {
		t.Errorf("did not expect owner-api block in --include fleet bundle")
	}
}

func TestCredsExport_BadInclude(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	if _, err := runExport(t, cfgPath, []string{"--out", filepath.Join(t.TempDir(), "b.enc"), "--include", "nonsense"}); err == nil {
		t.Fatal("expected usage error for unknown include token")
	} else if ExitCode(err) != 2 {
		t.Errorf("expected exit 2 for usage error, got %d (%v)", ExitCode(err), err)
	}
}

func TestCredsExport_UnwritableOut(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	// /nonexistent-dir-that-cant-be-created/foo doesn't exist; stat fails.
	bad := filepath.Join(t.TempDir(), "definitely-not-a-dir", "subdir", "bundle.enc")
	_, err := runExport(t, cfgPath, []string{"--out", bad})
	if err == nil {
		t.Fatal("expected unwritable-out usage error")
	}
	if ExitCode(err) != 2 {
		t.Errorf("expected exit 2 for unwritable --out, got %d (%v)", ExitCode(err), err)
	}
}

func TestCredsExport_VerifyMode_NoDiskWrite(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	// IsVerifyEnv reads PRINTING_PRESS_VERIFY=1; set it for this test only.
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	// Even with no passphrase fn substituted, verify-mode must short-circuit
	// BEFORE the prompt.
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("verify-mode export must not prompt for passphrase")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	stdout, err := runExport(t, cfgPath, []string{"--out", outPath, "--include", "keys,fleet,owner-api"})
	if err != nil {
		t.Fatalf("verify export: %v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("verify-mode wrote to disk; stat err: %v", err)
	}
	if !strings.Contains(stdout, `"verify_noop"`) {
		t.Errorf("expected verify_noop in envelope, got %q", stdout)
	}
}

func TestCredsExport_DryRun_NoDiskWrite(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	// dryRun is a flag, not env. Build flags directly.
	flags := &rootFlags{configPath: cfgPath, asJSON: true, dryRun: true}
	cmd := newCredsExportCmd(flags)
	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--out", outPath, "--include", "keys,fleet"})
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("dry-run export must not prompt for passphrase")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })

	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run export: %v", err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote to disk; stat err: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run"`) {
		t.Errorf("expected dry_run in envelope, got %q", out.String())
	}
}

func TestCredsExport_NoPlaintextPassphraseInBundle(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	if _, err := runExport(t, cfgPath, []string{"--out", outPath, "--include", "keys,fleet,owner-api"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	if bytes.Contains(raw, []byte(testCredsPassphrase)) {
		t.Error("bundle contains plaintext passphrase!")
	}
}

func TestCredsExport_NoPlaintextKeyBytesInBundle(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	if _, err := runExport(t, cfgPath, []string{"--out", outPath, "--include", "keys,fleet"}); err != nil {
		t.Fatalf("export: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	// The synthetic key bytes from setupFullyConfiguredCfg include this sentinel.
	if bytes.Contains(raw, []byte("FAKEKEYBYTES_TESLA_PP_TEST_SENTINEL")) {
		t.Error("bundle leaks plaintext private key bytes!")
	}
}

func TestCredsExport_NoPromptViaEnv(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	t.Setenv(credsPassphraseEnvVar, testCredsPassphrase)
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("--no-prompt must not call readPassphraseFn")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	if _, err := runExport(t, cfgPath, []string{"--out", outPath, "--no-prompt"}); err != nil {
		t.Fatalf("no-prompt export: %v", err)
	}
}

func TestCredsExport_NoPromptWithoutEnv(t *testing.T) {
	cfgPath, _ := setupFullyConfiguredCfg(t)
	// Ensure the env var is NOT set.
	t.Setenv(credsPassphraseEnvVar, "")
	if v := os.Getenv(credsPassphraseEnvVar); v != "" {
		// Setenv with "" on some platforms leaves the var set; explicitly unset.
		_ = os.Unsetenv(credsPassphraseEnvVar)
	}
	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	_, err := runExport(t, cfgPath, []string{"--out", outPath, "--no-prompt"})
	if err == nil {
		t.Fatal("expected usage error for --no-prompt with no env var")
	}
	if ExitCode(err) != 2 {
		t.Errorf("expected exit 2, got %d (%v)", ExitCode(err), err)
	}
}

func TestCredsExport_KeysButNoKeyPath(t *testing.T) {
	// Fresh config with no fleet private key path.
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// Fleet block with no PrivateKeyPath.
	if err := cfg.SaveFleetTokens("c", "s", "a", "r", time.Now().Add(time.Hour), "d", ""); err != nil {
		t.Fatalf("save: %v", err)
	}
	withPassphrase(t, testCredsPassphrase)

	outPath := filepath.Join(t.TempDir(), "bundle.tgz.enc")
	_, err = runExport(t, cfgPath, []string{"--out", outPath, "--include", "keys,fleet"})
	if err == nil {
		t.Fatal("expected usage error when keys requested but no key path")
	}
	if ExitCode(err) != 2 {
		t.Errorf("expected exit 2 for missing key path, got %d", ExitCode(err))
	}
}

// decryptForTest is a test-only helper that exercises the same decrypt path
// the import command uses. Tests can inspect contents without invoking the
// import command (which has its own refusal logic that would muddy the
// export-side assertions).
func decryptForTest(t *testing.T, bundlePath, passphrase string) *credsBundleContents {
	t.Helper()
	salt, nonce, ciphertext, err := readBundleFile(bundlePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	key := deriveCredsKey([]byte(passphrase), salt)
	plaintext, err := decryptCredsBundle(key, nonce, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	contents, err := unpackCredsBundle(plaintext)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	return contents
}

// Ensure cobra import stays used (some test files don't reach it directly).
var _ = cobra.Command{}
