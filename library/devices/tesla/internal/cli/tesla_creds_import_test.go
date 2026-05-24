// Tests for `tesla auth import`. See plan 2026-05-22-001 U2c.
//
// Test pattern: build a bundle via the export side, then exercise the import
// side against a fresh "destination" HOME directory. Same package-level
// readPassphraseFn shim used by the export tests.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// makeBundleForImport runs the export path against a fully-configured source
// machine and returns the bundle bytes. The "source machine" HOME and config
// live in tempdirs that go out of scope after the function returns; only the
// returned bundle bytes survive.
func makeBundleForImport(t *testing.T, includes string) (bundlePath, srcKeyPath, srcCfgPath string) {
	t.Helper()
	srcCfg, srcKey := setupFullyConfiguredCfg(t)
	withPassphrase(t, testCredsPassphrase)

	bundleDir := t.TempDir()
	bundlePath = filepath.Join(bundleDir, "bundle.tgz.enc")
	args := []string{"--out", bundlePath}
	if includes != "" {
		args = append(args, "--include", includes)
	}
	if _, err := runExport(t, srcCfg, args); err != nil {
		t.Fatalf("export: %v", err)
	}
	return bundlePath, srcKey, srcCfg
}

// runImport invokes the import command on a fresh destination HOME and config
// dir. The caller passes `extraArgs` to add --force etc.
func runImport(t *testing.T, bundlePath, destHome string, extraArgs ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", destHome)
	cfgDir := filepath.Join(destHome, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")

	flags := &rootFlags{configPath: cfgPath, asJSON: true}
	cmd := newCredsImportCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	args := append([]string{bundlePath}, extraArgs...)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCredsImport_RoundtripAllIncludes(t *testing.T) {
	bundlePath, srcKey, _ := makeBundleForImport(t, "keys,fleet,owner-api")
	srcKeyBytes, err := os.ReadFile(srcKey)
	if err != nil {
		t.Fatalf("read src key: %v", err)
	}

	destHome := t.TempDir()
	withPassphrase(t, testCredsPassphrase)
	if _, err := runImport(t, bundlePath, destHome); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Destination config should now have the fleet block + ios-app tokens.
	destCfg := filepath.Join(destHome, ".config", "tesla-pp-cli", "config.toml")
	cfg, err := config.Load(destCfg)
	if err != nil {
		t.Fatalf("load dest cfg: %v", err)
	}
	ft := cfg.FleetTokens()
	if ft.ClientID != "fleet-client-id-1" {
		t.Errorf("ClientID: got %q want fleet-client-id-1", ft.ClientID)
	}
	if ft.AccessToken != "fleet-access-1" {
		t.Errorf("AccessToken: got %q", ft.AccessToken)
	}
	if ft.RefreshToken != "fleet-refresh-1" {
		t.Errorf("RefreshToken: got %q", ft.RefreshToken)
	}
	if ft.PublicKeyDomain != "keys.example.com" {
		t.Errorf("PublicKeyDomain: got %q", ft.PublicKeyDomain)
	}
	// PrivateKeyPath should point at ~/.tesla/<name> on the dest machine.
	wantKeyPath := filepath.Join(destHome, ".tesla", "test-private.pem")
	if ft.PrivateKeyPath != wantKeyPath {
		t.Errorf("PrivateKeyPath: got %q want %q", ft.PrivateKeyPath, wantKeyPath)
	}
	// Key file landed with mode 0600 and contents matching source.
	info, err := os.Stat(wantKeyPath)
	if err != nil {
		t.Fatalf("stat dest key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("dest key mode = %o, want 0600", info.Mode().Perm())
	}
	gotKeyBytes, err := os.ReadFile(wantKeyPath)
	if err != nil {
		t.Fatalf("read dest key: %v", err)
	}
	if !bytes.Equal(srcKeyBytes, gotKeyBytes) {
		t.Errorf("dest key bytes != src key bytes")
	}
	// iOS-app tokens were applied.
	if cfg.AccessToken != "ios-access-1" {
		t.Errorf("ios AccessToken: got %q", cfg.AccessToken)
	}
	if cfg.RefreshToken != "ios-refresh-1" {
		t.Errorf("ios RefreshToken: got %q", cfg.RefreshToken)
	}
}

func TestCredsImport_PartialFleetOnly_LeavesOwnerAPIUntouched(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "fleet")

	destHome := t.TempDir()
	// Seed destination with an existing ios-app token; import of a
	// fleet-only bundle must NOT clobber it.
	t.Setenv("HOME", destHome)
	cfgDir := filepath.Join(destHome, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destCfgPath := filepath.Join(cfgDir, "config.toml")
	destCfg, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("load dest: %v", err)
	}
	if err := destCfg.SaveTokens("", "", "preexisting-ios-access", "preexisting-ios-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("save preexisting: %v", err)
	}

	withPassphrase(t, testCredsPassphrase)
	if _, err := runImport(t, bundlePath, destHome); err != nil {
		t.Fatalf("import: %v", err)
	}
	after, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.AccessToken != "preexisting-ios-access" {
		t.Errorf("ios-app AccessToken clobbered by fleet-only import: got %q", after.AccessToken)
	}
	if after.FleetTokens().ClientID != "fleet-client-id-1" {
		t.Errorf("fleet block not applied")
	}
}

func TestCredsImport_RefusesNonExpiredFleetToken(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")

	destHome := t.TempDir()
	t.Setenv("HOME", destHome)
	cfgDir := filepath.Join(destHome, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destCfgPath := filepath.Join(cfgDir, "config.toml")
	destCfg, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("load dest: %v", err)
	}
	// Plant a fresh fleet token at the destination.
	if err := destCfg.SaveFleetTokens(
		"existing-cid", "existing-secret",
		"existing-access", "existing-refresh",
		time.Now().Add(2*time.Hour),
		"existing-domain.example.com",
		"/tmp/existing-key.pem",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	withPassphrase(t, testCredsPassphrase)
	_, err = runImport(t, bundlePath, destHome)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if ExitCode(err) != 2 {
		t.Errorf("expected exit 2, got %d (%v)", ExitCode(err), err)
	}
	// Confirm nothing changed on disk.
	after, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.FleetTokens().AccessToken != "existing-access" {
		t.Errorf("fleet token was partially overwritten despite refusal: %q", after.FleetTokens().AccessToken)
	}
}

func TestCredsImport_ForceOverwritesExistingToken(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")

	destHome := t.TempDir()
	t.Setenv("HOME", destHome)
	cfgDir := filepath.Join(destHome, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destCfgPath := filepath.Join(cfgDir, "config.toml")
	destCfg, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("load dest: %v", err)
	}
	if err := destCfg.SaveFleetTokens("existing-cid", "existing-secret", "existing-access", "existing-refresh", time.Now().Add(2*time.Hour), "existing-domain.example.com", "/tmp/existing-key.pem"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	withPassphrase(t, testCredsPassphrase)
	if _, err := runImport(t, bundlePath, destHome, "--force"); err != nil {
		t.Fatalf("force import: %v", err)
	}
	after, err := config.Load(destCfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.FleetTokens().AccessToken != "fleet-access-1" {
		t.Errorf("--force did not overwrite token: %q", after.FleetTokens().AccessToken)
	}
}

func TestCredsImport_WrongPassphrase(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")
	destHome := t.TempDir()

	withPassphrase(t, "definitely-the-wrong-passphrase")
	stdout, err := runImport(t, bundlePath, destHome)
	if err == nil {
		t.Fatal("expected decryption error")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("error does not mention decryption failed: %v", err)
	}
	// No partial write — destination cfg should not exist (or at least no
	// fleet block).
	destCfgPath := filepath.Join(destHome, ".config", "tesla-pp-cli", "config.toml")
	if _, statErr := os.Stat(destCfgPath); statErr == nil {
		cfg, _ := config.Load(destCfgPath)
		if cfg.FleetTokens().AccessToken != "" {
			t.Errorf("partial write detected after wrong-passphrase: %q", cfg.FleetTokens().AccessToken)
		}
	}
	_ = stdout
}

func TestCredsImport_CorruptedBundle(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")

	// Truncate the last byte of the bundle to corrupt the GCM tag.
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(bundlePath, raw[:len(raw)-1], 0o600); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	destHome := t.TempDir()
	withPassphrase(t, testCredsPassphrase)
	_, err = runImport(t, bundlePath, destHome)
	if err == nil {
		t.Fatal("expected decryption error on corrupted bundle")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("expected 'decryption failed', got: %v", err)
	}
}

func TestCredsImport_MajorVersionMismatch(t *testing.T) {
	// Build a bundle with manifest.schema_version="99.0" by exercising the
	// pack/encrypt path directly (the export command always writes the
	// current constant).
	withPassphrase(t, testCredsPassphrase)
	cfgPath, _ := setupFullyConfiguredCfg(t)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	contents, err := buildCredsBundleContents(cfg, []string{"fleet"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	contents.Manifest.SchemaVersion = "99.0"
	plaintext, err := packCredsBundle(contents)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	salt := bytes.Repeat([]byte{0x42}, credsSaltLen)
	nonce := bytes.Repeat([]byte{0x24}, credsNonceLen)
	key := deriveCredsKey([]byte(testCredsPassphrase), salt)
	ciphertext, err := encryptCredsBundle(key, nonce, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	bundlePath := filepath.Join(t.TempDir(), "bundle.enc")
	if err := writeCredsBundleAtomic(bundlePath, salt, nonce, ciphertext); err != nil {
		t.Fatalf("write: %v", err)
	}

	destHome := t.TempDir()
	_, err = runImport(t, bundlePath, destHome)
	if err == nil {
		t.Fatal("expected refusal on major schema mismatch")
	}
	if ExitCode(err) != 2 {
		t.Errorf("expected exit 2, got %d (%v)", ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("error should mention schema_version: %v", err)
	}
}

func TestCredsImport_MinorVersionMismatchWarns(t *testing.T) {
	// Build a bundle with manifest.schema_version="1.99" (same major as
	// credsManifestSchemaVersion="1.0" but a future minor).
	withPassphrase(t, testCredsPassphrase)
	cfgPath, _ := setupFullyConfiguredCfg(t)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	contents, err := buildCredsBundleContents(cfg, []string{"fleet"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	contents.Manifest.SchemaVersion = "1.99"
	plaintext, err := packCredsBundle(contents)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	salt := bytes.Repeat([]byte{0x11}, credsSaltLen)
	nonce := bytes.Repeat([]byte{0x22}, credsNonceLen)
	key := deriveCredsKey([]byte(testCredsPassphrase), salt)
	ciphertext, err := encryptCredsBundle(key, nonce, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	bundlePath := filepath.Join(t.TempDir(), "bundle.enc")
	if err := writeCredsBundleAtomic(bundlePath, salt, nonce, ciphertext); err != nil {
		t.Fatalf("write: %v", err)
	}

	destHome := t.TempDir()
	stdout, err := runImport(t, bundlePath, destHome)
	if err != nil {
		t.Fatalf("minor mismatch should succeed with warning: %v", err)
	}
	if !strings.Contains(stdout, "schema_version_warning") {
		t.Errorf("expected schema_version_warning in envelope: %s", stdout)
	}
}

func TestCredsImport_VerifyMode_NoDiskWrite(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("verify-mode import must not prompt")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })

	destHome := t.TempDir()
	stdout, err := runImport(t, bundlePath, destHome)
	if err != nil {
		t.Fatalf("verify import: %v", err)
	}
	if !strings.Contains(stdout, `"verify_noop"`) {
		t.Errorf("expected verify_noop in envelope, got %s", stdout)
	}
	// No key file should exist.
	if _, err := os.Stat(filepath.Join(destHome, ".tesla", "test-private.pem")); !os.IsNotExist(err) {
		t.Errorf("verify-mode wrote key file; stat err: %v", err)
	}
}

func TestCredsImport_DryRun_NoDiskWrite(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")
	destHome := t.TempDir()
	t.Setenv("HOME", destHome)
	cfgDir := filepath.Join(destHome, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	destCfgPath := filepath.Join(cfgDir, "config.toml")
	flags := &rootFlags{configPath: destCfgPath, asJSON: true, dryRun: true}
	cmd := newCredsImportCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{bundlePath})
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("dry-run import must not prompt")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run import: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run"`) {
		t.Errorf("expected dry_run in envelope: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(destHome, ".tesla", "test-private.pem")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote key file; stat err: %v", err)
	}
}

func TestCredsImport_IntegrationLaptopToMacmini(t *testing.T) {
	// Full simulation: source HOME has full state, build a bundle, throw
	// the source HOME away, run import on a fresh destination HOME.
	bundlePath, _, srcCfgPath := makeBundleForImport(t, "keys,fleet,owner-api")

	// Capture source state so we can compare against destination state.
	srcCfg, err := config.Load(srcCfgPath)
	if err != nil {
		t.Fatalf("load src: %v", err)
	}
	srcFleet := srcCfg.FleetTokens()

	destHome := t.TempDir()
	withPassphrase(t, testCredsPassphrase)
	if _, err := runImport(t, bundlePath, destHome); err != nil {
		t.Fatalf("import: %v", err)
	}

	destCfg, err := config.Load(filepath.Join(destHome, ".config", "tesla-pp-cli", "config.toml"))
	if err != nil {
		t.Fatalf("load dest: %v", err)
	}
	destFleet := destCfg.FleetTokens()

	// Compare field-by-field, ignoring PrivateKeyPath (rehomed to ~/.tesla
	// on the destination machine — the rehoming is the explicit feature).
	if destFleet.ClientID != srcFleet.ClientID {
		t.Errorf("ClientID drift: %q vs %q", destFleet.ClientID, srcFleet.ClientID)
	}
	if destFleet.ClientSecret != srcFleet.ClientSecret {
		t.Errorf("ClientSecret drift")
	}
	if destFleet.AccessToken != srcFleet.AccessToken {
		t.Errorf("AccessToken drift")
	}
	if destFleet.RefreshToken != srcFleet.RefreshToken {
		t.Errorf("RefreshToken drift")
	}
	if !destFleet.TokenExpiry.Equal(srcFleet.TokenExpiry) {
		t.Errorf("TokenExpiry drift: %v vs %v", destFleet.TokenExpiry, srcFleet.TokenExpiry)
	}
	if destFleet.PublicKeyDomain != srcFleet.PublicKeyDomain {
		t.Errorf("PublicKeyDomain drift")
	}
	if destCfg.AccessToken != srcCfg.AccessToken {
		t.Errorf("ios AccessToken drift")
	}
}

func TestCredsImport_NoPromptViaEnv(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "fleet")
	destHome := t.TempDir()
	t.Setenv(credsPassphraseEnvVar, testCredsPassphrase)
	prev := readPassphraseFn
	readPassphraseFn = func(string) ([]byte, error) {
		t.Fatal("--no-prompt must not call readPassphraseFn")
		return nil, nil
	}
	t.Cleanup(func() { readPassphraseFn = prev })
	if _, err := runImport(t, bundlePath, destHome, "--no-prompt"); err != nil {
		t.Fatalf("no-prompt import: %v", err)
	}
}

func TestCredsImport_EnvelopeShape(t *testing.T) {
	bundlePath, _, _ := makeBundleForImport(t, "keys,fleet")
	destHome := t.TempDir()
	withPassphrase(t, testCredsPassphrase)
	stdout, err := runImport(t, bundlePath, destHome)
	if err != nil {
		t.Fatalf("import: %v (stdout=%q)", err, stdout)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("envelope is not JSON: %v\nstdout=%q", err, stdout)
	}
	if envelope["imported"] != true {
		t.Errorf("envelope.imported = %v, want true", envelope["imported"])
	}
}
