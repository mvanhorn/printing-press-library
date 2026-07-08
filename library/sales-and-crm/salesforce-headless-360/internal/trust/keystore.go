package trust

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// KeyRecord describes one registered public key for verification. Verifiers
// look up by KID. The real deployment path is to register the public key as
// a Salesforce Certificate or CMDT record in the org; the local keystore is
// the offline verification cache.
type KeyRecord struct {
	KID                 string     `json:"kid"`
	OrgAlias            string     `json:"org_alias"`
	OrgID               string     `json:"org_id,omitempty"`
	Algorithm           string     `json:"algorithm"`
	PublicKeyPEM        string     `json:"public_key_pem"`
	HostFingerprint     string     `json:"host_fingerprint,omitempty"`
	IssuerUserID        string     `json:"issuer_user_id,omitempty"`
	RegisteredAt        time.Time  `json:"registered_at"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	RetiredAt           *time.Time `json:"retired_at,omitempty"`
	RetiredReason       string     `json:"retired_reason,omitempty"`
	Source              string     `json:"source"` // "certificate", "cmdt", "local-generated"
	CertificateID       string     `json:"certificate_id,omitempty"`
	CMDTFullName        string     `json:"cmdt_full_name,omitempty"`
	PreviousReceiptHash string     `json:"previous_receipt_hash,omitempty"`
	Receipt             string     `json:"receipt,omitempty"`
}

// keystoreDir returns the directory holding registered key records.
func keystoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "pp", "salesforce-headless-360", "keystore"), nil
}

// SaveKeyRecord writes a KeyRecord under keystore/<kid>.json.
func SaveKeyRecord(r KeyRecord) error {
	if r.Algorithm == "" {
		r.Algorithm = "Ed25519"
	}
	dir, err := keystoreDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, r.KID+".json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Put stores a key record. The name mirrors the OS-keychain API this package
// will eventually use, while keeping the current file-backed implementation.
func Put(kid string, r KeyRecord) error {
	r.KID = kid
	return SaveKeyRecord(r)
}

// GetByKid retrieves a key record by KID.
func GetByKid(kid string) (*KeyRecord, error) {
	return LoadKeyRecord(kid)
}

// LoadKeyRecord looks up a record by KID. Returns os.ErrNotExist if missing.
func LoadKeyRecord(kid string) (*KeyRecord, error) {
	dir, err := keystoreDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, kid+".json"))
	if err != nil {
		return nil, err
	}
	var r KeyRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListKeyRecords returns all records in the keystore, most recent first.
func ListKeyRecords() ([]KeyRecord, error) {
	dir, err := keystoreDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var records []KeyRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r KeyRecord
		if json.Unmarshal(data, &r) == nil {
			records = append(records, r)
		}
	}
	return records, nil
}

// RetireKey marks a key as retired. Verification of bundles signed by this
// key will still succeed (for cached bundles) but new bundle generation
// refuses.
func RetireKey(kid string) error {
	r, err := LoadKeyRecord(kid)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	r.RetiredAt = &now
	return SaveKeyRecord(*r)
}

// RetireKeyWithReason marks a key as retired and preserves the reason for
// audit and list output.
func RetireKeyWithReason(kid, reason string) error {
	r, err := LoadKeyRecord(kid)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	r.RetiredAt = &now
	r.RetiredReason = reason
	return SaveKeyRecord(*r)
}

// ParsePublicKeyPEM parses a PEM-encoded Ed25519 public key.
func ParsePublicKeyPEM(pemStr string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ed, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 key")
	}
	return ed, nil
}
