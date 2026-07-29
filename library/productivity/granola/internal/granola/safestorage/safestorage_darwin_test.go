// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

//go:build darwin

package safestorage

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PATCH(dek-migration): coverage for the tier-1 scheme classification. All
// of it is darwin-only because the two-tier unwrap and the Keychain probe
// it classifies around live in safestorage_darwin.go.
//
// Every test here replaces the keychainEntry seam. Nothing in this package
// may shell out to /usr/bin/security: on a machine with a live Granola
// entry the test would assert against the operator's real state, and on a
// machine without one it would block on an approval prompt.

// testKeychainB64 is a synthetic stand-in for the "Granola Safe Storage"
// value - 16 zero bytes, which base64-encodes to the 24 characters
// fetchKeychainEntry validates.
const testKeychainB64 = "AAAAAAAAAAAAAAAAAAAAAA=="

func stubKeychain(t *testing.T, b64 string, err error) {
	t.Helper()
	prev := keychainEntry
	keychainEntry = func() (string, error) { return b64, err }
	t.Cleanup(func() { keychainEntry = prev })
}

// forbidKeychain asserts the classification settles a state from local
// files alone. Probing the Keychain for a machine with no Granola install
// would prompt the user to authorize a lookup that cannot help them.
func forbidKeychain(t *testing.T) {
	t.Helper()
	prev := keychainEntry
	keychainEntry = func() (string, error) {
		t.Error("Keychain must not be probed for this state")
		return "", errors.New("unreachable")
	}
	t.Cleanup(func() { keychainEntry = prev })
}

// withSupportDir points the resolver at a scratch support dir and clears
// the override so the loadDEK path (not the override short-circuit) runs.
func withSupportDir(t *testing.T) string {
	t.Helper()
	Reset()
	dir := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", dir)
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "")
	t.Cleanup(Reset)
	return dir
}

// writeStorageDEK builds a real Electron safeStorage v10 envelope around
// dek, keyed off testKeychainB64, so the tier-1 unwrap is exercised for
// real rather than stubbed.
func writeStorageDEK(t *testing.T, dir string, dek []byte) {
	t.Helper()
	key, err := pbkdf2.Key(sha1.New, testKeychainB64, []byte(pbkdf2Salt), pbkdf2Iters, pbkdf2KeyLen)
	if err != nil {
		t.Fatalf("pbkdf2: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	plain := []byte(base64.StdEncoding.EncodeToString(dek))
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, bytes.Repeat([]byte{cbcIVByte}, aes.BlockSize)).CryptBlocks(out, padded)
	blob := append([]byte(v10Prefix), out...)
	if err := os.WriteFile(filepath.Join(dir, "storage.dek"), blob, 0o600); err != nil {
		t.Fatalf("write storage.dek: %v", err)
	}
}

func writeEncryptedCache(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cache-v6.json.enc"), []byte("ciphertext"), 0o600); err != nil {
		t.Fatalf("write cache-v6.json.enc: %v", err)
	}
}

// TestLoadDEK_MigratedScheme is the state this unit exists for: Granola
// imported the DEK into its entitlement-gated access group and unlinked
// storage.dek, leaving the encrypted cache and the legacy Keychain entry
// behind.
func TestLoadDEK_MigratedScheme(t *testing.T) {
	dir := withSupportDir(t)
	writeEncryptedCache(t, dir)
	stubKeychain(t, testKeychainB64, nil)

	_, err := loadDEK()
	if err == nil {
		t.Fatal("loadDEK() should fail when storage.dek is gone")
	}
	if !errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("expected ErrSchemeMigrated, got: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{
		"Business or Enterprise",
		"remains readable",
		"GRANOLA_SAFESTORAGE_KEY_OVERRIDE",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("migrated-scheme error missing %q:\n%s", want, msg)
		}
	}
	// The whole point of the classification: stop blaming the local
	// install, and stop sending the user to remedies that cannot work.
	for _, unwanted := range []string{
		"not installed",
		"pre-encryption",
		"granola-pp-cli sync",
		"Always Allow",
	} {
		if strings.Contains(msg, unwanted) {
			t.Errorf("migrated-scheme error should not mention %q:\n%s", unwanted, msg)
		}
	}
}

// TestLoadDEK_ValidStorageDEK pins the unchanged two-tier unwrap: a
// machine that still has its key file behaves exactly as before.
func TestLoadDEK_ValidStorageDEK(t *testing.T) {
	dir := withSupportDir(t)
	writeEncryptedCache(t, dir)
	want, err := base64.StdEncoding.DecodeString(syntheticDEKBase64)
	if err != nil {
		t.Fatal(err)
	}
	writeStorageDEK(t, dir, want)
	stubKeychain(t, testKeychainB64, nil)

	got, err := loadDEK()
	if err != nil {
		t.Fatalf("loadDEK() error: %v", err)
	}
	if len(got) != dekLen {
		t.Fatalf("DEK length %d, expected %d", len(got), dekLen)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("DEK mismatch:\n got %x\n want %x", got, want)
	}
}

func TestLoadDEK_SupportDirAbsent(t *testing.T) {
	dir := withSupportDir(t)
	t.Setenv("GRANOLA_SUPPORT_DIR", filepath.Join(dir, "no-such-granola"))
	forbidKeychain(t)

	_, err := loadDEK()
	if !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("expected ErrKeyUnavailable, got: %v", err)
	}
	if errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("a missing install is not the migrated scheme: %v", err)
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected a not-installed error, got: %v", err)
	}
}

func TestLoadDEK_PreEncryptionInstall(t *testing.T) {
	withSupportDir(t)
	forbidKeychain(t)

	_, err := loadDEK()
	if !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("expected ErrKeyUnavailable, got: %v", err)
	}
	if errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("a pre-encryption install is not the migrated scheme: %v", err)
	}
	if !strings.Contains(err.Error(), "pre-encryption") {
		t.Errorf("expected a pre-encryption error, got: %v", err)
	}
}

func TestLoadDEK_EncryptedCacheButNoKeychainEntry(t *testing.T) {
	dir := withSupportDir(t)
	writeEncryptedCache(t, dir)
	// Exactly what fetchKeychainEntry returns for errSecItemNotFound.
	stubKeychain(t, "", errors.New("safestorage: Keychain key or DEK unavailable: Keychain entry not found (sign in to Granola desktop first)"))

	_, err := loadDEK()
	if err == nil {
		t.Fatal("loadDEK() should fail with no Keychain entry")
	}
	if errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("a signed-out install is not the migrated scheme: %v", err)
	}
	if !strings.Contains(err.Error(), "sign in to Granola desktop") {
		t.Errorf("expected the not-signed-in error, got: %v", err)
	}
}

func TestLoadDEK_MalformedEnvelope(t *testing.T) {
	dir := withSupportDir(t)
	writeEncryptedCache(t, dir)
	// "v10" plus a body that is not a whole number of AES blocks.
	if err := os.WriteFile(filepath.Join(dir, "storage.dek"), []byte(v10Prefix+"12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	stubKeychain(t, testKeychainB64, nil)

	_, err := loadDEK()
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed, got: %v", err)
	}
	if errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("a malformed envelope is not the migrated scheme: %v", err)
	}
	if !strings.Contains(err.Error(), "not a multiple of") {
		t.Errorf("expected the malformed-envelope error, got: %v", err)
	}
}

// TestDecrypt_StaleDEKFileAfterFailedMigration covers the install whose
// DEK import failed: storage.dek survived the migration but the app has
// already rotated to a Keychain-held key, so the file unwraps cleanly and
// then authenticates nothing.
func TestDecrypt_StaleDEKFileAfterFailedMigration(t *testing.T) {
	dir := withSupportDir(t)
	writeEncryptedCache(t, dir)
	// A well-formed key file holding a DEK that is not the one the
	// committed fixture was encrypted under.
	stale := bytes.Repeat([]byte{0xAB}, dekLen)
	writeStorageDEK(t, dir, stale)
	stubKeychain(t, testKeychainB64, nil)

	_, err := Decrypt(readFixture(t, "fixture-supabase.enc"))
	if err == nil {
		t.Fatal("Decrypt() should fail with a stale DEK")
	}
	if !errors.Is(err, ErrSchemeMigrated) {
		t.Fatalf("expected ErrSchemeMigrated for a stale key file, got: %v", err)
	}
	if !strings.Contains(err.Error(), "stale key file") {
		t.Errorf("expected the stale-key-file wording, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Business or Enterprise") {
		t.Errorf("stale-key-file error should carry the same remedy text, got: %v", err)
	}
}
