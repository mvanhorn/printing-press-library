// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestLabelAndLedgerFilesUsePrivateModesAndSafeNames(t *testing.T) {
	root := filepath.Join(t.TempDir(), "labels")
	encoded := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nsynthetic"))
	labelPath := writeLabelPDF(root, "../unsafe/tracking", encoded)
	if labelPath == "" {
		t.Fatal("writeLabelPDF failed")
	}
	if filepath.Dir(labelPath) != root {
		t.Fatalf("label escaped root: %s", labelPath)
	}
	if got := fileMode(t, root); got != 0o700 {
		t.Fatalf("label dir mode=%#o, want 0700", got)
	}
	if got := fileMode(t, labelPath); got != 0o600 {
		t.Fatalf("label mode=%#o, want 0600", got)
	}

	ledgerPath := filepath.Join(t.TempDir(), "state", "ledger.csv")
	if err := writeShipBulkLedger(ledgerPath, []shipBulkResult{{OrderID: "order-1", Status: "PASS"}}); err != nil {
		t.Fatalf("writeShipBulkLedger: %v", err)
	}
	if got := fileMode(t, filepath.Dir(ledgerPath)); got != 0o700 {
		t.Fatalf("ledger dir mode=%#o, want 0700", got)
	}
	if got := fileMode(t, ledgerPath); got != 0o600 {
		t.Fatalf("ledger mode=%#o, want 0600", got)
	}
}

func TestWriteLabelPDFRejectsSymlinkTarget(t *testing.T) {
	root := filepath.Join(t.TempDir(), "labels")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.pdf")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "123.pdf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\nreplacement"))
	if got := writeLabelPDF(root, "123", encoded); got != "" {
		t.Fatalf("writeLabelPDF accepted symlink target: %s", got)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("symlink target modified: %q", data)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
