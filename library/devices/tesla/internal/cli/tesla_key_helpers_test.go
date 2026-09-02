package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// generateTestKeyPair creates a temporary ECDSA P-256 keypair and writes
// the private and public keys to <basename>-private.pem and <basename>-public.pem
// in the given directory. Returns paths to both files.
func generateTestKeyPair(t *testing.T, dir, basename string) (privPath, pubPath string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	privPath = filepath.Join(dir, basename+"-private.pem")
	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	pubPath = filepath.Join(dir, basename+"-public.pem")
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		t.Fatalf("write public key: %v", err)
	}
	return privPath, pubPath
}

// ---------------------------------------------------------------------------
// validatePrivateKeyPEM tests
// ---------------------------------------------------------------------------

func TestValidatePrivateKeyPEM_ValidKey(t *testing.T) {
	dir := t.TempDir()
	privPath, _ := generateTestKeyPair(t, dir, "test")
	if err := validatePrivateKeyPEM(privPath); err != nil {
		t.Errorf("validatePrivateKeyPEM on valid key: %v", err)
	}
}

func TestValidatePrivateKeyPEM_Directory_Rejected(t *testing.T) {
	dir := t.TempDir()
	if err := validatePrivateKeyPEM(dir); err == nil {
		t.Errorf("validatePrivateKeyPEM on directory should fail")
	} else if !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("error should mention 'not a regular file', got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_Missing_Rejected(t *testing.T) {
	if err := validatePrivateKeyPEM("/no/such/path/key.pem"); err == nil {
		t.Errorf("validatePrivateKeyPEM on missing path should fail")
	} else if !strings.Contains(err.Error(), "cannot stat") {
		t.Errorf("error should mention 'cannot stat', got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_Unreadable_Rejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission test is POSIX-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable.pem")
	if err := os.WriteFile(path, []byte("dummy"), 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	if err := validatePrivateKeyPEM(path); err == nil {
		t.Errorf("validatePrivateKeyPEM on unreadable file should fail")
	} else if !strings.Contains(err.Error(), "cannot read") {
		t.Errorf("error should mention 'cannot read', got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_NoPEMBlock_Rejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-pem.pem")
	if err := os.WriteFile(path, []byte("not a pem file"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := validatePrivateKeyPEM(path); err == nil {
		t.Errorf("validatePrivateKeyPEM on non-PEM should fail")
	} else if !strings.Contains(err.Error(), "no PEM block") {
		t.Errorf("error should mention 'no PEM block', got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_PublicKeyPEM_Rejected(t *testing.T) {
	dir := t.TempDir()
	_, pubPath := generateTestKeyPair(t, dir, "test")
	if err := validatePrivateKeyPEM(pubPath); err == nil {
		t.Errorf("validatePrivateKeyPEM on public key should fail")
	} else if !strings.Contains(err.Error(), "not a private key") {
		t.Errorf("error should mention 'not a private key', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// selectKeyByPublicMatch tests
// ---------------------------------------------------------------------------

func TestSelectKeyByPublicMatch_SingleKey(t *testing.T) {
	dir := t.TempDir()
	priv1, _ := generateTestKeyPair(t, dir, "key1")
	candidates := []string{priv1}
	// With only one key and no target, selectKeyByPublicMatch returns ""
	// (because it only matches via sibling when no target is specified).
	// The single-key fast path is handled by the caller.
	result := selectKeyByPublicMatch(candidates, "")
	if result != priv1 {
		t.Errorf("single key with sibling should match, got: %q", result)
	}
}

func TestSelectKeyByPublicMatch_MatchesSiblingPublicKey(t *testing.T) {
	dir := t.TempDir()
	priv1, _ := generateTestKeyPair(t, dir, "key1")
	priv2, _ := generateTestKeyPair(t, dir, "key2")
	// Both have siblings, so both would match — should return "" (ambiguous).
	candidates := []string{priv1, priv2}
	result := selectKeyByPublicMatch(candidates, "")
	if result != "" {
		t.Errorf("two keys with siblings should be ambiguous, got: %q", result)
	}

	// Remove key2's public sibling, now only key1 should match.
	os.Remove(filepath.Join(dir, "key2-public.pem"))
	result = selectKeyByPublicMatch(candidates, "")
	if result != priv1 {
		t.Errorf("only key1 has sibling, should match, got: %q", result)
	}
}

func TestSelectKeyByPublicMatch_ExplicitTarget(t *testing.T) {
	dir := t.TempDir()
	priv1, pub1 := generateTestKeyPair(t, dir, "key1")
	priv2, _ := generateTestKeyPair(t, dir, "key2")
	candidates := []string{priv1, priv2}
	// Match against explicit target public key.
	result := selectKeyByPublicMatch(candidates, pub1)
	if result != priv1 {
		t.Errorf("should match key1 against its public key, got: %q", result)
	}
}

func TestSelectKeyByPublicMatch_DecoyDefaultName_NotSelected(t *testing.T) {
	dir := t.TempDir()
	// Create the "default" filename key that should NOT be selected just by name.
	decoyPriv, _ := generateTestKeyPair(t, dir, "tesla-keys-host")
	// Create another key with a proper sibling.
	realPriv, realPub := generateTestKeyPair(t, dir, "my-fleet-host")
	// Remove the decoy's public sibling so it can't match.
	os.Remove(filepath.Join(dir, "tesla-keys-host-public.pem"))

	candidates := []string{decoyPriv, realPriv}
	// Without target, should match the one with a sibling (realPriv).
	result := selectKeyByPublicMatch(candidates, "")
	if result != realPriv {
		t.Errorf("should select key with sibling, not decoy default name, got: %q", result)
	}

	// With explicit target for the real key, should still select realPriv.
	result = selectKeyByPublicMatch(candidates, realPub)
	if result != realPriv {
		t.Errorf("with explicit target, should select real key, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// scanValidPrivateKeys tests
// ---------------------------------------------------------------------------

func TestScanValidPrivateKeys_FiltersInvalid(t *testing.T) {
	dir := t.TempDir()
	validPriv, _ := generateTestKeyPair(t, dir, "valid")

	// Create an invalid "private key" that's actually a directory.
	invalidDir := filepath.Join(dir, "fake-private.pem")
	if err := os.Mkdir(invalidDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create an invalid PEM file (wrong content).
	invalidPEM := filepath.Join(dir, "broken-private.pem")
	if err := os.WriteFile(invalidPEM, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	candidates := scanValidPrivateKeys(dir)
	if len(candidates) != 1 || candidates[0] != validPriv {
		t.Errorf("should only find valid key, got: %v", candidates)
	}
}

// ---------------------------------------------------------------------------
// errMultipleCandidates tests
// ---------------------------------------------------------------------------

func TestErrMultipleCandidates_Format(t *testing.T) {
	err := errMultipleCandidates("/home/.tesla", []string{"/home/.tesla/a.pem", "/home/.tesla/b.pem"}, "Use --key-file")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	for _, want := range []string{"/home/.tesla", "a.pem", "b.pem", "Use --key-file"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should contain %q, got: %s", want, msg)
		}
	}
}
