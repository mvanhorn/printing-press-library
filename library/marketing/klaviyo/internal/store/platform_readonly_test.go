// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestVerifiedHealthDoesNotUpdateReadOnlyGateMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	owner := klaviyoStoreSession(time.Unix(1, 0).UTC())
	st, err := OpenVerifiedWithContext(context.Background(), path, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := InspectResourceHealth(path, klaviyoStoreSession(time.Unix(2, 0).UTC())); err != nil {
		t.Fatalf("doctor health attempted to mutate read-only metadata: %v", err)
	}
}

func klaviyoStoreSession(checkedAt time.Time) *platform.Session {
	return &platform.Session{
		ProfileName: "tenant-a", Source: "klaviyo", GateOutcome: platform.GateVerified,
		ExpectedIdentity:      map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
		ObservedIdentity:      map[string]string{"account_id": "acct-a", "org_name": "Tenant A"},
		CredentialFingerprint: "fixture-fingerprint", GateCheckedAt: checkedAt,
	}
}
