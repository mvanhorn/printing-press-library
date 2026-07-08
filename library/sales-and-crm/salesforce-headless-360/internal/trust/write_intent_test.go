package trust

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerifyWriteIntentRejectsBundleAudience(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writeIntentTestSigner(t)
	claims := fixedWriteIntentClaims()
	claims.Aud = "agent-context"
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	jws, err := SignJWS(signer, payload)
	if err != nil {
		t.Fatalf("SignJWS: %v", err)
	}

	_, err = VerifyWriteIntent([]byte(jws))
	if !errors.Is(err, ErrWrongAudience) {
		t.Fatalf("VerifyWriteIntent error = %v, want %v", err, ErrWrongAudience)
	}
}

func TestVerifyWriteIntentRejectsTamperedDiff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writeIntentTestSigner(t)
	jws, err := signer.SignWriteIntent(fixedWriteIntentClaims())
	if err != nil {
		t.Fatalf("SignWriteIntent: %v", err)
	}

	parts := strings.Split(string(jws), ".")
	var claims WriteIntentClaims
	payload, err := b64urlDecode(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	claims.DiffSha256 = strings.Repeat("b", 64)
	tamperedPayload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal tampered claims: %v", err)
	}
	tampered := parts[0] + "." + b64url(tamperedPayload) + "." + parts[2]

	_, err = VerifyWriteIntent([]byte(tampered))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("VerifyWriteIntent error = %v, want %v", err, ErrSignatureInvalid)
	}
}

func TestSignWriteIntentRoundTrips(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writeIntentTestSigner(t)
	claims := fixedWriteIntentClaims()

	jws, err := signer.SignWriteIntent(claims)
	if err != nil {
		t.Fatalf("SignWriteIntent: %v", err)
	}

	verified, err := VerifyWriteIntent(jws)
	if err != nil {
		t.Fatalf("VerifyWriteIntent: %v", err)
	}
	if verified != claims {
		t.Fatalf("claims mismatch:\n got: %#v\nwant: %#v", verified, claims)
	}
}

func TestVerifyWriteIntentRejectsUnknownKID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writeIntentTestSigner(t)
	jws, err := signer.SignWriteIntent(fixedWriteIntentClaims())
	if err != nil {
		t.Fatalf("SignWriteIntent: %v", err)
	}
	if err := RetireKey(signer.KID()); err != nil {
		t.Fatalf("RetireKey setup sanity: %v", err)
	}
	t.Setenv("HOME", t.TempDir())

	_, err = VerifyWriteIntent(jws)
	if !errors.Is(err, ErrUnknownKID) {
		t.Fatalf("VerifyWriteIntent error = %v, want %v", err, ErrUnknownKID)
	}
}

func TestVerifyWriteIntentRejectsExpiredIntent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writeIntentTestSigner(t)
	claims := fixedWriteIntentClaims()
	claims.Exp = time.Now().UTC().Add(-time.Minute).Unix()

	jws, err := signer.SignWriteIntent(claims)
	if err != nil {
		t.Fatalf("SignWriteIntent: %v", err)
	}

	_, err = VerifyWriteIntent(jws)
	if !errors.Is(err, ErrIntentExpired) {
		t.Fatalf("VerifyWriteIntent error = %v, want %v", err, ErrIntentExpired)
	}
}

func writeIntentTestSigner(t *testing.T) *FileSigner {
	t.Helper()
	signer, err := NewFileSignerWithIdentity("prod", "host123456", "005TEST")
	if err != nil {
		t.Fatalf("NewFileSignerWithIdentity: %v", err)
	}
	if err := SaveKeyRecord(KeyRecord{
		KID:             signer.KID(),
		OrgAlias:        "prod",
		OrgID:           "00D000000000001",
		Algorithm:       "Ed25519",
		PublicKeyPEM:    signer.PublicKeyPEM(),
		HostFingerprint: "host123456",
		IssuerUserID:    "005TEST",
		RegisteredAt:    time.Now().UTC(),
		Source:          "local-generated",
	}); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}
	return signer
}

func fixedWriteIntentClaims() WriteIntentClaims {
	now := time.Now().UTC()
	return WriteIntentClaims{
		Iss:            "00D000000000001",
		Sub:            "005000000000001",
		Aud:            WriteIntentAudience,
		Iat:            now.Add(-time.Minute).Unix(),
		Exp:            now.Add(5 * time.Minute).Unix(),
		Jti:            "01JWRITEINTENT",
		SObject:        "Account",
		RecordID:       "001000000000001",
		Operation:      "update",
		DiffSha256:     strings.Repeat("a", 64),
		IdempotencyKey: "idem-123",
		IfLastModified: "2026-04-22T14:00:00Z",
	}
}
