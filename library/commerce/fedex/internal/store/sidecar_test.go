// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsPreexistingSQLiteSidecarSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fedex.db")
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("sentinel"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, path+"-wal"); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if state, err := Open(path); err == nil {
		state.Close()
		t.Fatal("Open accepted a pre-existing WAL symlink")
	}
}

func TestSQLiteSidecarsArePrivateWhileOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fedex.db")
	state, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer state.Close()
	if _, err := state.InsertShipment(t.Context(), Shipment{TrackingNumber: "synthetic-tracking"}); err != nil {
		t.Fatalf("InsertShipment: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Stat(path + suffix)
		if err != nil {
			t.Fatalf("stat %s: %v", suffix, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode=%#o, want 0600", suffix, got)
		}
	}
}
