// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

//go:build !darwin

package safestorage

import (
	"encoding/base64"
	"fmt"
)

// loadDEK is the non-darwin stub. Linux libsecret and Windows DPAPI
// implementations are deferred to follow-up work; see plan
// docs/plans/2026-05-12-001-feat-granola-encrypted-cache-plan.md
// "Out of scope (Deferred to Follow-Up Work)".
func loadDEK() ([]byte, error) {
	return nil, ErrUnsupportedPlatform
}

// PATCH(dek-migration): classifyStaleDEKFile is the non-darwin stub for
// the shared Decrypt path. There is no local key file to go stale on a
// platform where loadDEK never produces one, so nothing is reclassified
// and Decrypt keeps returning ErrDecryptFailed. Present so the shared
// code referencing ErrSchemeMigrated still compiles off darwin.
func classifyStaleDEKFile() error {
	return nil
}

// parseKeyOverride parses GRANOLA_SAFESTORAGE_KEY_OVERRIDE. The env
// override works cross-platform so non-darwin CI lanes can still
// exercise Decrypt with a known DEK.
func parseKeyOverride(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(raw) != dekLen {
		return nil, fmt.Errorf("decoded length %d, expected %d", len(raw), dekLen)
	}
	return raw, nil
}
