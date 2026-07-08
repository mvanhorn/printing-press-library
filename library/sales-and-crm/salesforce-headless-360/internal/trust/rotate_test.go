package trust

import (
	"testing"
	"time"
)

func TestRotateRegistersNewKeyAndRetiresOld(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	opts := fixedRegisterOptions()
	if _, err := RegisterKey(&fakeOrgClient{}, opts); err != nil {
		t.Fatalf("RegisterKey: %v", err)
	}

	now := time.Date(2026, 4, 22, 13, 0, 0, 0, time.UTC)
	client := &fakeOrgClient{}
	result, err := RotateKey(client, RotateOptions{
		OrgAlias:        opts.OrgAlias,
		OrgID:           opts.OrgID,
		HostFingerprint: opts.HostFingerprint,
		UserID:          opts.UserID,
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}
	if result.OldKID == "" {
		t.Fatal("expected old kid")
	}
	if result.New == nil || result.New.KID == result.OldKID {
		t.Fatalf("expected new distinct key: %#v", result)
	}
	old, err := LoadKeyRecord(result.OldKID)
	if err != nil {
		t.Fatalf("LoadKeyRecord old: %v", err)
	}
	if old.RetiredAt == nil {
		t.Fatal("old key was not retired")
	}
	if _, err := GetSignerByKid(result.OldKID); err != nil {
		t.Fatalf("old private key should remain retrievable by kid: %v", err)
	}
	if _, err := GetSignerByKid(result.New.KID); err != nil {
		t.Fatalf("new private key should be retrievable by kid: %v", err)
	}
}
