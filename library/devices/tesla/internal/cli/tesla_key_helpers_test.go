package cli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
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

func TestValidatePrivateKeyPEM_RSAPrivateKey_Rejected(t *testing.T) {
	dir := t.TempDir()
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA: %v", err)
	}
	pkcs1Path := filepath.Join(dir, "rsa-pkcs1.pem")
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})
	if err := os.WriteFile(pkcs1Path, pkcs1PEM, 0o600); err != nil {
		t.Fatalf("write PKCS1 RSA: %v", err)
	}
	if err := validatePrivateKeyPEM(pkcs1Path); err == nil {
		t.Errorf("validatePrivateKeyPEM on RSA PRIVATE KEY should fail")
	} else if !strings.Contains(err.Error(), "ECDSA") {
		t.Errorf("error should mention ECDSA, got: %v", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatalf("marshal PKCS8 RSA: %v", err)
	}
	pkcs8Path := filepath.Join(dir, "rsa-pkcs8.pem")
	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	if err := os.WriteFile(pkcs8Path, pkcs8PEM, 0o600); err != nil {
		t.Fatalf("write PKCS8 RSA: %v", err)
	}
	if err := validatePrivateKeyPEM(pkcs8Path); err == nil {
		t.Errorf("validatePrivateKeyPEM on PKCS#8 RSA should fail")
	} else if !strings.Contains(err.Error(), "ECDSA") {
		t.Errorf("error should mention ECDSA, got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_ECPrivateKeyPEM_Accepted(t *testing.T) {
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sec1, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal EC: %v", err)
	}
	path := filepath.Join(dir, "ec-sec1.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}), 0o600); err != nil {
		t.Fatalf("write EC PRIVATE KEY: %v", err)
	}
	if err := validatePrivateKeyPEM(path); err != nil {
		t.Errorf("validatePrivateKeyPEM on EC PRIVATE KEY: %v", err)
	}
}

func TestValidatePrivateKeyPEM_P384_Rejected(t *testing.T) {
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal P-384 key: %v", err)
	}
	path := filepath.Join(dir, "p384-private.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), 0o600); err != nil {
		t.Fatalf("write P-384 key: %v", err)
	}
	if err := validatePrivateKeyPEM(path); err == nil {
		t.Errorf("validatePrivateKeyPEM should reject P-384 key")
	} else if !strings.Contains(err.Error(), "P-384") || !strings.Contains(err.Error(), "P-256") {
		t.Errorf("error should mention P-384 curve and P-256 requirement, got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_P521_Rejected(t *testing.T) {
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-521 key: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal P-521 key: %v", err)
	}
	path := filepath.Join(dir, "p521-private.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), 0o600); err != nil {
		t.Fatalf("write P-521 key: %v", err)
	}
	if err := validatePrivateKeyPEM(path); err == nil {
		t.Errorf("validatePrivateKeyPEM should reject P-521 key")
	} else if !strings.Contains(err.Error(), "P-521") || !strings.Contains(err.Error(), "P-256") {
		t.Errorf("error should mention P-521 curve and P-256 requirement, got: %v", err)
	}
}

func TestValidatePrivateKeyPEM_P384_EC_PRIVATE_KEY_Rejected(t *testing.T) {
	dir := t.TempDir()
	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	sec1, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal P-384 EC key: %v", err)
	}
	path := filepath.Join(dir, "p384-ec-private.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: sec1}), 0o600); err != nil {
		t.Fatalf("write P-384 EC key: %v", err)
	}
	if err := validatePrivateKeyPEM(path); err == nil {
		t.Errorf("validatePrivateKeyPEM should reject P-384 EC PRIVATE KEY")
	} else if !strings.Contains(err.Error(), "P-384") || !strings.Contains(err.Error(), "P-256") {
		t.Errorf("error should mention P-384 curve and P-256 requirement, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// selectKeyByPublicMatch tests
// ---------------------------------------------------------------------------

func TestSelectKeyByPublicMatch_EmptyTarget_NeverAutoSelects(t *testing.T) {
	dir := t.TempDir()
	priv1, _ := generateTestKeyPair(t, dir, "key1")
	// Single candidate with a sibling still does not auto-select; callers
	// take the single-key fast path themselves.
	if result := selectKeyByPublicMatch([]string{priv1}, ""); result != "" {
		t.Errorf("empty targetPubPath must not auto-select, got: %q", result)
	}

	priv2, _ := generateTestKeyPair(t, dir, "key2")
	os.Remove(filepath.Join(dir, "key2-public.pem"))
	// Only key1 has a self-matching sibling; that must not win.
	if result := selectKeyByPublicMatch([]string{priv1, priv2}, ""); result != "" {
		t.Errorf("empty targetPubPath must not select sibling pair, got: %q", result)
	}
}

func TestSelectKeyByPublicMatch_ExplicitTarget(t *testing.T) {
	dir := t.TempDir()
	priv1, pub1 := generateTestKeyPair(t, dir, "key1")
	priv2, _ := generateTestKeyPair(t, dir, "key2")
	candidates := []string{priv1, priv2}
	result := selectKeyByPublicMatch(candidates, pub1)
	if result != priv1 {
		t.Errorf("should match key1 against its public key, got: %q", result)
	}
}

func TestSelectKeyByPublicMatch_DecoySibling_EmptyTarget_NotSelected(t *testing.T) {
	dir := t.TempDir()
	// Unrelated BLE/manual pair with a matching sibling public PEM.
	decoyPriv, _ := generateTestKeyPair(t, dir, "ble-manual")
	// Fleet-template key: move its public PEM to a domain-named path so the
	// private file has no sibling in the directory (the Fleet-template shape).
	fleetPriv, fleetPub := generateTestKeyPair(t, dir, "fleet-host")
	registeredPub := filepath.Join(dir, "example.com-public.pem")
	if err := os.Rename(fleetPub, registeredPub); err != nil {
		t.Fatalf("rename registered public key: %v", err)
	}

	candidates := []string{decoyPriv, fleetPriv}
	if result := selectKeyByPublicMatch(candidates, ""); result != "" {
		t.Errorf("empty targetPubPath must not select decoy sibling pair, got: %q", result)
	}

	result := selectKeyByPublicMatch(candidates, registeredPub)
	if result != fleetPriv {
		t.Errorf("with explicit target, should select fleet key, got: %q", result)
	}
}

// TestSelectKeyByPublicMatch_SoleCandidate_Mismatch verifies that a single
// candidate that doesn't match the target public key returns empty string.
// This is critical for fleet-register: if ~/.tesla has exactly one key but
// it's unrelated to the registered domain, we must NOT persist it.
func TestSelectKeyByPublicMatch_SoleCandidate_Mismatch(t *testing.T) {
	dir := t.TempDir()

	// Create an unrelated key (old BLE key, different pair).
	unrelatedPriv, _ := generateTestKeyPair(t, dir, "old-ble-key")

	// Create a "domain" public key that doesn't match unrelatedPriv.
	_, domainPub := generateTestKeyPair(t, dir, "registered-domain")

	// Single candidate that doesn't match the target public key.
	candidates := []string{unrelatedPriv}
	result := selectKeyByPublicMatch(candidates, domainPub)
	if result != "" {
		t.Errorf("sole candidate with mismatched public key should return empty, got: %q", result)
	}
}

// TestSelectKeyByPublicMatch_SoleCandidate_Match verifies that a single
// candidate that DOES match the target public key is returned.
func TestSelectKeyByPublicMatch_SoleCandidate_Match(t *testing.T) {
	dir := t.TempDir()

	// Create a key pair and use its public key as the "domain" public key.
	matchingPriv, matchingPub := generateTestKeyPair(t, dir, "matching-key")

	candidates := []string{matchingPriv}
	result := selectKeyByPublicMatch(candidates, matchingPub)
	if result != matchingPriv {
		t.Errorf("sole candidate matching public key should be returned, got: %q", result)
	}
}

func TestSelectKeyByPublicBytes_DuplicateCopies_SelectsOne(t *testing.T) {
	dir := t.TempDir()
	priv, pub := generateTestKeyPair(t, dir, "z-copy")
	data, err := os.ReadFile(priv)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	copyPath := filepath.Join(dir, "a-copy-private.pem")
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		t.Fatalf("write key copy: %v", err)
	}
	target := readPublicKeyBytes(pub)
	// Unsorted input: original path first. Stable choice is sorted-first copy.
	got := selectKeyByPublicBytes([]string{priv, copyPath}, target)
	if got != copyPath {
		t.Errorf("duplicate matching copies should return sorted-first path, got %q want %q", got, copyPath)
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
