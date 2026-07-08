package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const GenesisReceiptHash = "GENESIS"

var (
	ErrReceiptChainBroken      = errors.New("RECEIPT_CHAIN_BROKEN")
	ErrReceiptSignatureInvalid = errors.New("RECEIPT_SIGNATURE_INVALID")
)

// ReceiptPayload is the canonical payload signed by the key being registered.
type ReceiptPayload struct {
	KID                 string    `json:"kid"`
	PublicKeyPEM        string    `json:"public_key_pem"`
	IssuerUserID        string    `json:"issuer_user_id"`
	RegisteredAt        time.Time `json:"registered_at"`
	PreviousReceiptHash string    `json:"previous_receipt_hash"`
}

// Receipt stores the signed proof of possession for a CMDT fallback key.
type Receipt struct {
	Payload   ReceiptPayload `json:"payload"`
	Signature string         `json:"signature"`
}

// NewReceipt creates a signed hash-chain receipt with the new key.
func NewReceipt(signer Signer, payload ReceiptPayload) (string, error) {
	if signer == nil {
		return "", fmt.Errorf("signer required")
	}
	if payload.KID == "" {
		payload.KID = signer.KID()
	}
	if payload.PreviousReceiptHash == "" {
		payload.PreviousReceiptHash = GenesisReceiptHash
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal receipt payload: %w", err)
	}
	jws, err := SignJWS(signer, payloadJSON)
	if err != nil {
		return "", err
	}
	receipt := Receipt{Payload: payload, Signature: jws}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return "", fmt.Errorf("marshal receipt: %w", err)
	}
	return string(receiptJSON), nil
}

// ReceiptHash returns the sha256 hex digest of a full receipt field.
func ReceiptHash(receipt string) string {
	sum := sha256.Sum256([]byte(receipt))
	return hex.EncodeToString(sum[:])
}

// VerifyReceipt validates one receipt's signature and payload consistency.
func VerifyReceipt(receiptField string) (*Receipt, error) {
	var receipt Receipt
	if err := json.Unmarshal([]byte(receiptField), &receipt); err != nil {
		return nil, fmt.Errorf("%w: parse receipt: %v", ErrReceiptSignatureInvalid, err)
	}
	pub, err := ParsePublicKeyPEM(receipt.Payload.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: parse public key: %v", ErrReceiptSignatureInvalid, err)
	}
	payload, header, err := VerifyJWS(receipt.Signature, pub)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReceiptSignatureInvalid, err)
	}
	if header["kid"] != receipt.Payload.KID {
		return nil, fmt.Errorf("%w: kid header mismatch", ErrReceiptSignatureInvalid)
	}
	canonicalPayload, err := json.Marshal(receipt.Payload)
	if err != nil {
		return nil, err
	}
	if string(payload) != string(canonicalPayload) {
		return nil, fmt.Errorf("%w: payload mismatch", ErrReceiptSignatureInvalid)
	}
	return &receipt, nil
}

// VerifyReceiptChain walks CMDT records from genesis and rejects overwritten or
// injected records whose previous_receipt_hash no longer matches the prior
// full receipt field.
func VerifyReceiptChain(records []KeyRecord) error {
	if len(records) == 0 {
		return nil
	}
	ordered := append([]KeyRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].RegisteredAt.Before(ordered[j].RegisteredAt)
	})
	expectedPrevious := GenesisReceiptHash
	for _, record := range ordered {
		if record.Receipt == "" {
			return fmt.Errorf("%w: missing receipt for %s", ErrReceiptChainBroken, record.KID)
		}
		receipt, err := VerifyReceipt(record.Receipt)
		if err != nil {
			return err
		}
		if receipt.Payload.KID != record.KID ||
			receipt.Payload.PublicKeyPEM != record.PublicKeyPEM ||
			receipt.Payload.IssuerUserID != record.IssuerUserID {
			return fmt.Errorf("%w: receipt payload does not match record %s", ErrReceiptChainBroken, record.KID)
		}
		if !receipt.Payload.RegisteredAt.Equal(record.RegisteredAt) {
			return fmt.Errorf("%w: registered_at mismatch for %s", ErrReceiptChainBroken, record.KID)
		}
		if receipt.Payload.PreviousReceiptHash != expectedPrevious {
			return fmt.Errorf("%w: previous hash mismatch for %s", ErrReceiptChainBroken, record.KID)
		}
		if record.PreviousReceiptHash != "" && record.PreviousReceiptHash != receipt.Payload.PreviousReceiptHash {
			return fmt.Errorf("%w: record previous hash mismatch for %s", ErrReceiptChainBroken, record.KID)
		}
		expectedPrevious = ReceiptHash(record.Receipt)
	}
	return nil
}
