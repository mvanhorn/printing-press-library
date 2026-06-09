// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the store meta(key,value) table used by the MCP pseudonymizer to
// persist its per-store HMAC salt (and any future small key/value state).
package store

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestMetaTableRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Missing key -> (nil, false, nil).
	got, ok, err := s.GetMeta("absent")
	if err != nil {
		t.Fatalf("GetMeta absent: %v", err)
	}
	if ok {
		t.Errorf("GetMeta absent: ok = true, want false")
	}
	if got != nil {
		t.Errorf("GetMeta absent: value = %v, want nil", got)
	}

	// Round-trip a binary value (salt-shaped).
	val := []byte{0x00, 0x01, 0xff, 0x7f, 0x80, 'a', 'b'}
	if err := s.SetMeta("salt", val); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	got, ok, err = s.GetMeta("salt")
	if err != nil {
		t.Fatalf("GetMeta salt: %v", err)
	}
	if !ok {
		t.Fatalf("GetMeta salt: ok = false, want true")
	}
	if !bytes.Equal(got, val) {
		t.Errorf("GetMeta salt round-trip = %v, want %v", got, val)
	}

	// Overwrite (upsert).
	val2 := []byte("second")
	if err := s.SetMeta("salt", val2); err != nil {
		t.Fatalf("SetMeta overwrite: %v", err)
	}
	got, _, err = s.GetMeta("salt")
	if err != nil {
		t.Fatalf("GetMeta after overwrite: %v", err)
	}
	if !bytes.Equal(got, val2) {
		t.Errorf("GetMeta after overwrite = %q, want %q", got, val2)
	}
}
