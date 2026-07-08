package trust

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReceiptChainValidatesThreeRecords(t *testing.T) {
	records := buildReceiptChain(t, 3)
	if err := VerifyReceiptChain(records); err != nil {
		t.Fatalf("VerifyReceiptChain: %v", err)
	}
}

func TestTamperedReceiptSignatureFails(t *testing.T) {
	records := buildReceiptChain(t, 1)
	var receipt Receipt
	if err := json.Unmarshal([]byte(records[0].Receipt), &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}
	parts := strings.Split(receipt.Signature, ".")
	receipt.Signature = parts[0] + "." + b64url([]byte(`{"kid":"attacker"}`)) + "." + parts[2]
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	records[0].Receipt = string(data)
	if err := VerifyReceiptChain(records); err == nil || !strings.Contains(err.Error(), ErrReceiptSignatureInvalid.Error()) {
		t.Fatalf("expected %s, got %v", ErrReceiptSignatureInvalid, err)
	}
}

func buildReceiptChain(t *testing.T, count int) []KeyRecord {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	previous := GenesisReceiptHash
	records := make([]KeyRecord, 0, count)
	for i := 0; i < count; i++ {
		signer, err := GenerateFileSignerWithIdentity("prod", "host123456", "005TEST")
		if err != nil {
			t.Fatalf("GenerateFileSignerWithIdentity: %v", err)
		}
		registeredAt := now.Add(time.Duration(i) * time.Minute)
		record := KeyRecord{
			KID:                 signer.KID(),
			OrgAlias:            "prod",
			Algorithm:           "Ed25519",
			PublicKeyPEM:        signer.PublicKeyPEM(),
			HostFingerprint:     "host123456",
			IssuerUserID:        "005TEST",
			RegisteredAt:        registeredAt,
			Source:              "cmdt",
			PreviousReceiptHash: previous,
		}
		receipt, err := NewReceipt(signer, ReceiptPayload{
			KID:                 record.KID,
			PublicKeyPEM:        record.PublicKeyPEM,
			IssuerUserID:        record.IssuerUserID,
			RegisteredAt:        record.RegisteredAt,
			PreviousReceiptHash: previous,
		})
		if err != nil {
			t.Fatalf("NewReceipt: %v", err)
		}
		record.Receipt = receipt
		previous = ReceiptHash(receipt)
		records = append(records, record)
	}
	return records
}
