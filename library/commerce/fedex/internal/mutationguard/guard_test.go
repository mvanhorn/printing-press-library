// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mutationguard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDuplicateWindowExpires(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	ledger := &Ledger{Dir: t.TempDir(), Window: time.Hour, Now: func() time.Time { return now }}
	body := []byte(`{"a":1}`)
	digest, err := ledger.Begin("https://apis.fedex.com", "POST", "/ship/v1/shipments", body, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Complete(digest, StatusSucceeded); err != nil {
		t.Fatal(err)
	}
	var duplicate *DuplicateError
	if _, err := ledger.Begin("https://apis.fedex.com", "POST", "/ship/v1/shipments", body, false); !errors.As(err, &duplicate) {
		t.Fatalf("within window err=%v, want DuplicateError", err)
	}
	now = now.Add(2 * time.Hour)
	if _, err := ledger.Begin("https://apis.fedex.com", "POST", "/ship/v1/shipments", body, false); err != nil {
		t.Fatalf("after window err=%v, want nil", err)
	}
}

func TestDigestSeparatesTargetsAndCanonicalizesBody(t *testing.T) {
	a, _ := Digest("https://apis.fedex.com", "POST", "/ship/v1/shipments", []byte(`{"b":2,"a":1}`))
	b, _ := Digest("https://apis.fedex.com/", "post", "/ship/v1/shipments", []byte(`{"a":1, "b":2}`))
	if a != b {
		t.Fatal("equivalent requests produced different digests")
	}
	for _, other := range [][3]string{
		{"https://apis-sandbox.fedex.com", "POST", "/ship/v1/shipments"},
		{"https://apis.fedex.com", "PUT", "/ship/v1/shipments"},
		{"https://apis.fedex.com", "POST", "/pickup/v1/pickups"},
	} {
		d, _ := Digest(other[0], other[1], other[2], []byte(`{"a":1,"b":2}`))
		if d == a {
			t.Fatalf("digest collided across targets: %v", other)
		}
	}
}

func TestLedgerStoresNoRequestBody(t *testing.T) {
	dir := t.TempDir()
	ledger := &Ledger{Dir: dir}
	secret := "SENTINEL-RECIPIENT-STREET"
	digest, err := ledger.Begin("https://apis.fedex.com", "POST", "/ship/v1/shipments", []byte(`{"street":"`+secret+`"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, digest+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatal("ledger persisted request body content")
	}
	info, err := os.Stat(filepath.Join(dir, digest+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("ledger entry mode=%v, want 0600", info.Mode().Perm())
	}
}

func TestStaleLockIsReclaimed(t *testing.T) {
	dir := t.TempDir()
	ledger := &Ledger{Dir: dir}
	body := []byte(`{"a":1}`)
	digest, _ := Digest("https://apis.fedex.com", "POST", "/x", body)
	lockPath := filepath.Join(dir, digest+".lock")
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Begin("https://apis.fedex.com", "POST", "/x", body, false); !errors.Is(err, ErrBusy) {
		t.Fatalf("fresh lock err=%v, want ErrBusy", err)
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Begin("https://apis.fedex.com", "POST", "/x", body, false); err != nil {
		t.Fatalf("stale lock err=%v, want nil", err)
	}
}

func TestRejectedForcedResendKeepsEarlierSuccessBlocking(t *testing.T) {
	ledger := &Ledger{Dir: t.TempDir()}
	body := []byte(`{"a":1}`)
	begin := func(force bool) (string, error) {
		return ledger.Begin("https://apis.fedex.com", "POST", "/ship/v1/shipments", body, force)
	}
	digest, err := begin(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Complete(digest, StatusSucceeded); err != nil {
		t.Fatal(err)
	}
	// A forced resend that FedEx rejects (429, 4xx) must not unblock the
	// original shipment's duplicate protection.
	digest, err = begin(true)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Complete(digest, StatusRejected); err != nil {
		t.Fatal(err)
	}
	var duplicate *DuplicateError
	if _, err := begin(false); !errors.As(err, &duplicate) || duplicate.Status != StatusSucceeded {
		t.Fatalf("ordinary resend after rejected forced attempt err=%v, want succeeded DuplicateError", err)
	}
}

func TestRejectedWriteDoesNotBlock(t *testing.T) {
	ledger := &Ledger{Dir: t.TempDir()}
	body := []byte(`{"a":1}`)
	digest, err := ledger.Begin("https://apis.fedex.com", "PUT", "/x", body, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Complete(digest, StatusRejected); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Begin("https://apis.fedex.com", "PUT", "/x", body, false); err != nil {
		t.Fatalf("resend after rejection err=%v, want nil", err)
	}
}
