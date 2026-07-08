package trust

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
)

const APIVersion = "v63.0"

type OrgClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Post(path string, body any) (json.RawMessage, int, error)
	Patch(path string, body any) (json.RawMessage, int, error)
}

type RegisterOptions struct {
	OrgAlias        string
	OrgID           string
	HostFingerprint string
	UserID          string
	ForceNew        bool
	Now             func() time.Time
}

type RegisterResult struct {
	KID           string    `json:"kid"`
	OrgAlias      string    `json:"org"`
	Source        string    `json:"source"`
	CertificateID string    `json:"certificate_id,omitempty"`
	CMDTFullName  string    `json:"cmdt_full_name,omitempty"`
	RegisteredAt  time.Time `json:"registered_at"`
	Idempotent    bool      `json:"idempotent"`
}

func RegisterKey(c OrgClient, opts RegisterOptions) (*RegisterResult, error) {
	opts = normalizeRegisterOptions(opts)
	if opts.OrgAlias == "" {
		return nil, fmt.Errorf("org alias required")
	}
	if !opts.ForceNew {
		record, err := activeDeviceRecord(opts.OrgAlias, opts.HostFingerprint, opts.UserID)
		if err != nil {
			return nil, err
		}
		if record != nil {
			return &RegisterResult{
				KID:           record.KID,
				OrgAlias:      record.OrgAlias,
				Source:        record.Source,
				CertificateID: record.CertificateID,
				CMDTFullName:  record.CMDTFullName,
				RegisteredAt:  record.RegisteredAt,
				Idempotent:    true,
			}, nil
		}
	}

	var signer *FileSigner
	var err error
	if opts.ForceNew {
		signer, err = GenerateFileSignerWithIdentity(opts.OrgAlias, opts.HostFingerprint, opts.UserID)
	} else {
		signer, err = NewFileSignerWithIdentity(opts.OrgAlias, opts.HostFingerprint, opts.UserID)
	}
	if err != nil {
		return nil, err
	}

	record := KeyRecord{
		KID:             signer.KID(),
		OrgAlias:        opts.OrgAlias,
		OrgID:           opts.OrgID,
		Algorithm:       "Ed25519",
		PublicKeyPEM:    signer.PublicKeyPEM(),
		HostFingerprint: opts.HostFingerprint,
		IssuerUserID:    opts.UserID,
		RegisteredAt:    opts.Now().UTC(),
	}

	if c == nil {
		record.Source = "local-generated"
		if err := SaveKeyRecord(record); err != nil {
			return nil, err
		}
		_ = RecordAuditEvent("register", record.KID, record.OrgAlias, record.Source, "")
		return registerResult(record, false), nil
	}

	certificateID, err := createCertificate(c, record)
	if err == nil {
		record.Source = "certificate"
		record.CertificateID = certificateID
		if err := SaveKeyRecord(record); err != nil {
			return nil, err
		}
		_ = RecordAuditEvent("register", record.KID, record.OrgAlias, record.Source, certificateID)
		return registerResult(record, false), nil
	}
	if !isCertificateUnavailable(err) {
		return nil, err
	}

	if err := registerCMDT(c, signer, &record); err != nil {
		return nil, err
	}
	if err := SaveKeyRecord(record); err != nil {
		return nil, err
	}
	_ = RecordAuditEvent("register", record.KID, record.OrgAlias, record.Source, record.CMDTFullName)
	return registerResult(record, false), nil
}

func normalizeRegisterOptions(opts RegisterOptions) RegisterOptions {
	identity := LocalIdentity()
	if opts.HostFingerprint == "" {
		opts.HostFingerprint = identity.HostFingerprint
	}
	if opts.UserID == "" {
		opts.UserID = identity.UserID
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return opts
}

func activeDeviceRecord(orgAlias, hostFingerprint, userID string) (*KeyRecord, error) {
	records, err := ListKeyRecords()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.OrgAlias == orgAlias &&
			record.HostFingerprint == hostFingerprint &&
			record.IssuerUserID == userID &&
			record.RetiredAt == nil {
			copy := record
			return &copy, nil
		}
	}
	return nil, nil
}

func createCertificate(c OrgClient, record KeyRecord) (string, error) {
	body := map[string]any{
		"Name":            certificateName(record.KID),
		"DeveloperName":   certificateName(record.KID),
		"MasterLabel":     "SF360 Bundle Key " + record.KID,
		"CertificateData": base64.StdEncoding.EncodeToString([]byte(record.PublicKeyPEM)),
	}
	raw, _, err := c.Post("/services/data/"+APIVersion+"/tooling/sobjects/Certificate", body)
	if err != nil {
		return "", err
	}
	var resp struct {
		ID      string   `json:"id"`
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse certificate response: %w", err)
	}
	if !resp.Success || resp.ID == "" {
		return "", fmt.Errorf("certificate create failed: %v", resp.Errors)
	}
	return resp.ID, nil
}

func registerCMDT(c OrgClient, signer Signer, record *KeyRecord) error {
	previous := previousCMDTReceiptHash(record.OrgAlias)
	receipt, err := NewReceipt(signer, ReceiptPayload{
		KID:                 record.KID,
		PublicKeyPEM:        record.PublicKeyPEM,
		IssuerUserID:        record.IssuerUserID,
		RegisteredAt:        record.RegisteredAt,
		PreviousReceiptHash: previous,
	})
	if err != nil {
		return err
	}
	record.Source = "cmdt"
	record.PreviousReceiptHash = previous
	record.Receipt = receipt
	record.CMDTFullName = "SF360_Bundle_Key." + cmdtDeveloperName(record.KID)

	body := map[string]any{
		"DeveloperName":          cmdtDeveloperName(record.KID),
		"MasterLabel":            "SF360 Bundle Key " + record.KID,
		"KID__c":                 record.KID,
		"PublicKeyPem__c":        record.PublicKeyPEM,
		"IssuerUserId__c":        record.IssuerUserID,
		"RegisteredAt__c":        record.RegisteredAt.Format(time.RFC3339),
		"PreviousReceiptHash__c": record.PreviousReceiptHash,
		"Receipt__c":             record.Receipt,
	}
	_, _, err = c.Post("/services/data/"+APIVersion+"/tooling/sobjects/SF360_Bundle_Key__mdt", body)
	return err
}

func previousCMDTReceiptHash(orgAlias string) string {
	records, err := ListKeyRecords()
	if err != nil {
		return GenesisReceiptHash
	}
	var latest *KeyRecord
	for i := range records {
		record := records[i]
		if record.OrgAlias != orgAlias || record.Source != "cmdt" || record.Receipt == "" {
			continue
		}
		if latest == nil || latest.RegisteredAt.Before(record.RegisteredAt) {
			latest = &record
		}
	}
	if latest == nil {
		return GenesisReceiptHash
	}
	return ReceiptHash(latest.Receipt)
}

func isCertificateUnavailable(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode != 400 && apiErr.StatusCode != 404 {
		return false
	}
	msg := strings.ToUpper(err.Error())
	// INVALID_FIELD / "No such column" covers Tooling API Certificate schema
	// drift (e.g. v63.0 dropping CertificateData) — fall through to CMDT
	// just like a missing sobject type. Observed live: HTTP 400 "No such
	// column 'CertificateData' on sobject of type Certificate" (F-022).
	return strings.Contains(msg, "INVALID_TYPE") || strings.Contains(msg, "NOT_FOUND") ||
		strings.Contains(msg, "INVALID_FIELD") || strings.Contains(msg, "NO SUCH COLUMN")
}

func registerResult(record KeyRecord, idempotent bool) *RegisterResult {
	return &RegisterResult{
		KID:           record.KID,
		OrgAlias:      record.OrgAlias,
		Source:        record.Source,
		CertificateID: record.CertificateID,
		CMDTFullName:  record.CMDTFullName,
		RegisteredAt:  record.RegisteredAt,
		Idempotent:    idempotent,
	}
}

func certificateName(kid string) string {
	return "SF360_Bundle_Key_" + cmdtDeveloperName(kid)
}

func cmdtDeveloperName(kid string) string {
	name := strings.NewReplacer("-", "_").Replace(kid)
	name = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, name)
	if len(name) > 35 {
		name = name[:35]
	}
	return name
}
