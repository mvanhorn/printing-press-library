package trust

import (
	"fmt"
	"time"
)

type RevokeOptions struct {
	KID    string
	Reason string
	Now    func() time.Time
}

type RevokeResult struct {
	KID           string    `json:"kid"`
	OrgAlias      string    `json:"org"`
	Status        string    `json:"status"`
	RetiredAt     time.Time `json:"retired_at"`
	RetiredReason string    `json:"retired_reason,omitempty"`
}

func RevokeKey(c OrgClient, opts RevokeOptions) (*RevokeResult, error) {
	if opts.KID == "" {
		return nil, fmt.Errorf("kid required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	record, err := LoadKeyRecord(opts.KID)
	if err != nil {
		return nil, err
	}
	retiredAt := opts.Now().UTC()
	record.RetiredAt = &retiredAt
	record.RetiredReason = opts.Reason
	if c != nil {
		if err := patchRetirement(c, *record); err != nil {
			return nil, err
		}
	}
	if err := SaveKeyRecord(*record); err != nil {
		return nil, err
	}
	_ = RecordAuditEvent("revoke", record.KID, record.OrgAlias, record.Source, opts.Reason)
	return &RevokeResult{
		KID:           record.KID,
		OrgAlias:      record.OrgAlias,
		Status:        "retired",
		RetiredAt:     retiredAt,
		RetiredReason: opts.Reason,
	}, nil
}

func patchRetirement(c OrgClient, record KeyRecord) error {
	body := map[string]any{
		"RetiredAt__c":     record.RetiredAt.Format(time.RFC3339),
		"RetiredReason__c": record.RetiredReason,
	}
	if record.Source == "certificate" && record.CertificateID != "" {
		_, _, err := c.Patch("/services/data/"+APIVersion+"/tooling/sobjects/Certificate/"+record.CertificateID, body)
		return err
	}
	if record.Source == "cmdt" {
		fullName := record.CMDTFullName
		if fullName == "" {
			fullName = "SF360_Bundle_Key." + cmdtDeveloperName(record.KID)
		}
		_, _, err := c.Patch("/services/data/"+APIVersion+"/tooling/sobjects/SF360_Bundle_Key__mdt/"+fullName, body)
		return err
	}
	return nil
}
