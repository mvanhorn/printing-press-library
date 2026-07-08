package trust

import (
	"testing"
	"time"
)

func TestRevokeKeyMarksRecordRetiredWithReason(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	result, err := RegisterKey(&fakeOrgClient{}, fixedRegisterOptions())
	if err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}
	now := time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC)
	revoke, err := RevokeKey(&fakeOrgClient{}, RevokeOptions{
		KID:    result.KID,
		Reason: "compromised laptop",
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}
	if revoke.Status != "retired" {
		t.Fatalf("status = %s", revoke.Status)
	}
	record, err := LoadKeyRecord(result.KID)
	if err != nil {
		t.Fatalf("LoadKeyRecord: %v", err)
	}
	if record.RetiredAt == nil || !record.RetiredAt.Equal(now) {
		t.Fatalf("retired_at = %v", record.RetiredAt)
	}
	if record.RetiredReason != "compromised laptop" {
		t.Fatalf("reason = %q", record.RetiredReason)
	}
}
