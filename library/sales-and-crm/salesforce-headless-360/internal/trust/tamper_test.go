package trust

import (
	"strings"
	"testing"
)

func TestReceiptChainDetectsOverwrite(t *testing.T) {
	records := buildReceiptChain(t, 3)

	attackerSigner, err := GenerateFileSignerWithIdentity("prod", "attackerhost", "005TEST")
	if err != nil {
		t.Fatalf("GenerateFileSignerWithIdentity: %v", err)
	}
	attacker := records[1]
	attacker.KID = attackerSigner.KID()
	attacker.PublicKeyPEM = attackerSigner.PublicKeyPEM()
	attacker.HostFingerprint = "attackerhost"
	attacker.PreviousReceiptHash = GenesisReceiptHash
	attacker.Receipt, err = NewReceipt(attackerSigner, ReceiptPayload{
		KID:                 attacker.KID,
		PublicKeyPEM:        attacker.PublicKeyPEM,
		IssuerUserID:        attacker.IssuerUserID,
		RegisteredAt:        attacker.RegisteredAt,
		PreviousReceiptHash: GenesisReceiptHash,
	})
	if err != nil {
		t.Fatalf("NewReceipt: %v", err)
	}
	records[1] = attacker

	err = VerifyReceiptChain(records)
	if err == nil || !strings.Contains(err.Error(), ErrReceiptChainBroken.Error()) {
		t.Fatalf("expected receipt chain broken from overwritten receipt, got %v", err)
	}
}

func TestReceiptChainDetectsWrongPreviousHash(t *testing.T) {
	records := buildReceiptChain(t, 3)
	records[1].PreviousReceiptHash = "bad-hash"
	err := VerifyReceiptChain(records)
	if err == nil || !strings.Contains(err.Error(), ErrReceiptChainBroken.Error()) {
		t.Fatalf("expected chain broken, got %v", err)
	}
}

func TestManifestBitFlipFailsSignatureVerification(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer, err := NewFileSignerWithIdentity("prod", "host123456", "005TEST")
	if err != nil {
		t.Fatalf("NewFileSignerWithIdentity: %v", err)
	}
	jws, err := SignJWS(signer, []byte(`{"manifest_sha256":"abc123"}`))
	if err != nil {
		t.Fatalf("SignJWS: %v", err)
	}
	parts := strings.Split(jws, ".")
	tampered := parts[0] + "." + b64url([]byte(`{"manifest_sha256":"xbc123"}`)) + "." + parts[2]
	_, _, err = VerifyJWS(tampered, signer.PublicKeyBytes())
	if err == nil || !strings.Contains(err.Error(), ErrSignatureInvalid.Error()) {
		t.Fatalf("expected signature invalid, got %v", err)
	}
}
