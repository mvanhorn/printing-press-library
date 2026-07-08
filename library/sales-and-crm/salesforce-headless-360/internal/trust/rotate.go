package trust

import (
	"fmt"
	"time"
)

type RotateOptions struct {
	OrgAlias        string
	OrgID           string
	HostFingerprint string
	UserID          string
	Reason          string
	Now             func() time.Time
}

type RotateResult struct {
	OldKID string          `json:"old_kid,omitempty"`
	New    *RegisterResult `json:"new"`
}

func RotateKey(c OrgClient, opts RotateOptions) (*RotateResult, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	registerOpts := normalizeRegisterOptions(RegisterOptions{
		OrgAlias:        opts.OrgAlias,
		OrgID:           opts.OrgID,
		HostFingerprint: opts.HostFingerprint,
		UserID:          opts.UserID,
		Now:             opts.Now,
	})
	if registerOpts.OrgAlias == "" {
		return nil, fmt.Errorf("org alias required")
	}

	old, err := activeDeviceRecord(registerOpts.OrgAlias, registerOpts.HostFingerprint, registerOpts.UserID)
	if err != nil {
		return nil, err
	}

	newResult, err := RegisterKey(c, RegisterOptions{
		OrgAlias:        registerOpts.OrgAlias,
		OrgID:           registerOpts.OrgID,
		HostFingerprint: registerOpts.HostFingerprint,
		UserID:          registerOpts.UserID,
		ForceNew:        true,
		Now:             opts.Now,
	})
	if err != nil {
		return nil, err
	}

	result := &RotateResult{New: newResult}
	if old == nil {
		_ = RecordAuditEvent("rotate", newResult.KID, newResult.OrgAlias, newResult.Source, "no previous active key")
		return result, nil
	}

	reason := opts.Reason
	if reason == "" {
		reason = "rotated"
	}
	if _, err := RevokeKey(c, RevokeOptions{KID: old.KID, Reason: reason, Now: opts.Now}); err != nil {
		return nil, err
	}
	result.OldKID = old.KID
	_ = RecordAuditEvent("rotate", newResult.KID, newResult.OrgAlias, newResult.Source, "retired "+old.KID)
	return result, nil
}
