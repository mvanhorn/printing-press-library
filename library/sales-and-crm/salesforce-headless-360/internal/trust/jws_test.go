package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRoundtrip exercises the full sign-then-verify flow, which is the
// canonical contract for bundle signing. A silent failure here means
// agents could trust tampered bundles.
func TestRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	signer, err := NewFileSigner("test-org")
	if err != nil {
		t.Fatalf("NewFileSigner: %v", err)
	}

	payload := []byte(`{"iss":"00D000000000001","sub":"005000000000001","jti":"01JEHX","manifest_sha256":"abc123"}`)
	jws, err := SignJWS(signer, payload)
	if err != nil {
		t.Fatalf("SignJWS: %v", err)
	}
	if strings.Count(jws, ".") != 2 {
		t.Fatalf("expected 3-segment JWS, got %s", jws)
	}

	kid, err := ExtractKIDUnsafe(jws)
	if err != nil {
		t.Fatalf("ExtractKIDUnsafe: %v", err)
	}
	if kid != signer.KID() {
		t.Fatalf("kid mismatch: header=%s signer=%s", kid, signer.KID())
	}

	verified, header, err := VerifyJWS(jws, signer.PublicKeyBytes())
	if err != nil {
		t.Fatalf("VerifyJWS: %v", err)
	}
	if string(verified) != string(payload) {
		t.Fatalf("payload mismatch: got %s", verified)
	}
	if header["alg"] != "EdDSA" {
		t.Fatalf("alg mismatch: got %v", header["alg"])
	}
}

// TestTamperedPayload asserts a bit-flip in the payload breaks verification.
// Without this guard, agents can silently trust forged bundles.
func TestTamperedPayload(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	signer, _ := NewFileSigner("test-org")
	payload := []byte(`{"iss":"A"}`)
	jws, _ := SignJWS(signer, payload)

	parts := strings.Split(jws, ".")
	// Replace payload with different content, keep signature
	tampered := parts[0] + "." + b64url([]byte(`{"iss":"X"}`)) + "." + parts[2]

	if _, _, err := VerifyJWS(tampered, signer.PublicKeyBytes()); err == nil {
		t.Fatal("expected verification to fail on tampered payload")
	}
}

// TestKeystoreRoundtrip proves SaveKeyRecord and LoadKeyRecord round-trip.
func TestKeystoreRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	rec := KeyRecord{
		KID:          "test-kid",
		OrgAlias:     "prod",
		PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\nfoo\n-----END PUBLIC KEY-----\n",
		Source:       "local-generated",
	}
	if err := SaveKeyRecord(rec); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}

	loaded, err := LoadKeyRecord("test-kid")
	if err != nil {
		t.Fatalf("LoadKeyRecord: %v", err)
	}
	if loaded.OrgAlias != "prod" {
		t.Fatalf("orgAlias mismatch: %s", loaded.OrgAlias)
	}

	// Confirm file permissions are restrictive
	path := filepath.Join(tmp, ".config", "pp", "salesforce-headless-360", "keystore", "test-kid.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 perms, got %v", info.Mode().Perm())
	}

	// Confirm it survives a full JSON round-trip
	data, _ := os.ReadFile(path)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["source"] != "local-generated" {
		t.Fatalf("source field lost: %v", raw)
	}
}

// TestRetireKey marks a key as retired and verifies the flag persists.
func TestRetireKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	rec := KeyRecord{KID: "kid-1", OrgAlias: "prod", PublicKeyPEM: "x", Source: "local-generated"}
	SaveKeyRecord(rec)

	if err := RetireKey("kid-1"); err != nil {
		t.Fatalf("RetireKey: %v", err)
	}
	loaded, _ := LoadKeyRecord("kid-1")
	if loaded.RetiredAt == nil {
		t.Fatal("expected RetiredAt to be set")
	}
}
