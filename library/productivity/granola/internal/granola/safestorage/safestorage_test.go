// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package safestorage

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticDEKBase64 matches testdata/synthetic-aes-gcm-test-dek.bin and is
// the value GRANOLA_SAFESTORAGE_KEY_OVERRIDE accepts for tests.
const syntheticDEKBase64 = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

func withDEKOverride(t *testing.T) {
	t.Helper()
	Reset()
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", syntheticDEKBase64)
	t.Cleanup(Reset)
}

func TestKey_UsesOverride(t *testing.T) {
	withDEKOverride(t)
	got, err := Key()
	if err != nil {
		t.Fatalf("Key() error: %v", err)
	}
	expect, _ := base64.StdEncoding.DecodeString(syntheticDEKBase64)
	if string(got) != string(expect) {
		t.Fatalf("Key() bytes mismatch:\n got %x\n want %x", got, expect)
	}
}

func TestKey_OverrideRejectsBadBase64(t *testing.T) {
	Reset()
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", "!!!not base64!!!")
	t.Cleanup(Reset)
	if _, err := Key(); err == nil {
		t.Fatal("Key() should fail on bad base64 override")
	}
}

func TestKey_OverrideRejectsWrongLength(t *testing.T) {
	Reset()
	short := base64.StdEncoding.EncodeToString([]byte("only sixteen byt"))
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", short)
	t.Cleanup(Reset)
	_, err := Key()
	if err == nil {
		t.Fatal("Key() should fail on 16-byte override")
	}
	if !strings.Contains(err.Error(), "expected 32") {
		t.Fatalf("expected 'expected 32' in error, got: %v", err)
	}
}

// PATCH(dek-migration): the override has to stay ahead of the scheme
// classification. It is not only a test seam - the upstream migration
// imports the existing DEK rather than generating a new one, so a
// pre-migration storage.dek recovered from a backup still decrypts
// today's files when supplied this way.
func TestKey_OverrideWinsWhenStorageDEKAbsent(t *testing.T) {
	t.Setenv("GRANOLA_SUPPORT_DIR", t.TempDir())
	withDEKOverride(t)
	got, err := Key()
	if err != nil {
		t.Fatalf("Key() error with override and no storage.dek: %v", err)
	}
	if len(got) != dekLen {
		t.Fatalf("Key() length %d, expected %d", len(got), dekLen)
	}
}

// TestSchemeMigratedError_AlsoReportsKeyUnavailable pins a deliberate
// relationship: the migrated-scheme error is a refinement of "key
// unavailable", not a replacement for it. Callers that only know the older
// sentinel (workos.go's plaintext supabase.json fallback) must keep
// matching, or adding the classification would silently remove a working
// path.
func TestSchemeMigratedError_AlsoReportsKeyUnavailable(t *testing.T) {
	err := newMigratedSchemeError("synthetic state")
	if !errors.Is(err, ErrSchemeMigrated) {
		t.Error("migrated-scheme error should match ErrSchemeMigrated")
	}
	if !errors.Is(err, ErrKeyUnavailable) {
		t.Error("migrated-scheme error should also match ErrKeyUnavailable")
	}
	if errors.Is(err, ErrDecryptFailed) {
		t.Error("migrated-scheme error should not match ErrDecryptFailed")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error message should be a single line, got: %q", err.Error())
	}
}

func TestDecrypt_FixtureSupabase(t *testing.T) {
	withDEKOverride(t)
	cipher := readFixture(t, "fixture-supabase.enc")
	plain, err := Decrypt(cipher)
	if err != nil {
		t.Fatalf("Decrypt(supabase fixture) error: %v", err)
	}
	want := "\"workos_tokens\""
	if !strings.Contains(string(plain), want) {
		t.Fatalf("plaintext missing %q. Head: %s", want, head(plain, 80))
	}
}

func TestDecrypt_FixtureCache(t *testing.T) {
	withDEKOverride(t)
	cipher := readFixture(t, "fixture-cache.enc")
	plain, err := Decrypt(cipher)
	if err != nil {
		t.Fatalf("Decrypt(cache fixture) error: %v", err)
	}
	for _, want := range []string{"\"cache\"", "\"state\"", "\"transcripts\""} {
		if !strings.Contains(string(plain), want) {
			t.Fatalf("plaintext missing %q. Head: %s", want, head(plain, 120))
		}
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	withDEKOverride(t)
	for _, n := range []int{0, 1, 12, 28} {
		buf := make([]byte, n)
		_, err := Decrypt(buf)
		if err == nil {
			t.Fatalf("Decrypt(%d-byte input) should fail", n)
		}
		if !errors.Is(err, ErrDecryptFailed) {
			t.Fatalf("Decrypt(%d-byte) returned %v, expected ErrDecryptFailed", n, err)
		}
	}
}

func TestDecrypt_CorruptTag(t *testing.T) {
	withDEKOverride(t)
	cipher := readFixture(t, "fixture-supabase.enc")
	cipher[len(cipher)-1] ^= 0xFF
	_, err := Decrypt(cipher)
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("Decrypt(corrupt tag) returned %v, expected ErrDecryptFailed", err)
	}
}

func TestDecrypt_CorruptMiddleByte(t *testing.T) {
	withDEKOverride(t)
	cipher := readFixture(t, "fixture-supabase.enc")
	cipher[40] ^= 0xFF
	_, err := Decrypt(cipher)
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("Decrypt(corrupt middle) returned %v, expected ErrDecryptFailed", err)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	Reset()
	wrong := base64.StdEncoding.EncodeToString(make([]byte, 32))
	t.Setenv("GRANOLA_SAFESTORAGE_KEY_OVERRIDE", wrong)
	t.Cleanup(Reset)
	cipher := readFixture(t, "fixture-supabase.enc")
	_, err := Decrypt(cipher)
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("Decrypt(wrong key) returned %v, expected ErrDecryptFailed", err)
	}
}

func TestAvailable_FalseBeforeKey(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	if Available() {
		t.Fatal("Available() should be false before any Key() call")
	}
}

// Note: Available() does not flip to true on the override path because
// successful override calls never populate the dekValue cache (the override
// short-circuits before the cache). Test that real loadDEK path populates
// the cache; documented behavior, not a regression.
func TestAvailable_OverrideDoesNotPopulateCache(t *testing.T) {
	withDEKOverride(t)
	if _, err := Key(); err != nil {
		t.Fatal(err)
	}
	if Available() {
		t.Fatal("Available() should remain false on the override path")
	}
}

func TestReset_ClearsCache(t *testing.T) {
	// Manually populate the cache to simulate a prior loadDEK success.
	dek, _ := base64.StdEncoding.DecodeString(syntheticDEKBase64)
	dekMu.Lock()
	dekValue = dek
	dekMu.Unlock()
	t.Cleanup(Reset)
	if !Available() {
		t.Fatal("setup failed: cache not populated")
	}
	Reset()
	if Available() {
		t.Fatal("Available() should be false after Reset")
	}
}

func TestZeroBytes(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5}
	ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("byte %d not zeroed: got %d", i, v)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func head(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}
