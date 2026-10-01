// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestMirrorDirRejectsDotIDs(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	for _, id := range []string{".", "..", ".hidden", "-x", "a/b", ""} {
		if _, err := actual.MirrorDir(id); err == nil {
			t.Errorf("MirrorDir(%q) accepted an unsafe id", id)
		}
	}
	if _, err := actual.MirrorDir("1cfdbb80-6274-49bf-b0c2-737235a4c81f"); err != nil {
		t.Errorf("MirrorDir rejected a real sync id: %v", err)
	}
}

// A file id resolves to the same mirror directory as its group id.
func TestPullByFileIDKeysMirrorOnGroupID(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	srv := fakeServer(t, budgetZip(t), nil, "")
	c := actual.New(srv.URL, 10*time.Second)
	res, err := actual.Pull(context.Background(), c, actual.PullOptions{Password: "hunter2", SyncID: "file-1"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := actual.DBPath("group-1")
	if res.DBPath != want || res.Manifest.SyncID != "group-1" {
		t.Fatalf("mirror keyed on %q (%s), want group-1 (%s)", res.Manifest.SyncID, res.DBPath, want)
	}
}

func TestServerErrorsRedactCredentials(t *testing.T) {
	c := actual.New("http://admin:s3cret@127.0.0.1:1", time.Second)
	_, err := c.ServerVersion(context.Background())
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("error leaks credentials: %v", err)
	}
}

func TestParseBankCSVMalformedFirstFieldDoesNotPanic(t *testing.T) {
	in := "Date,Amount,Description\n\"x\"y,1,Bad\n2026-09-01,-5.00,Kroger\n"
	txns, bad, err := actual.ParseBankCSV(strings.NewReader(in), actual.CSVMapping{DateCol: "Date", AmountCol: "Amount", PayeeCol: "Description"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 1 || bad[0].Line != 2 {
		t.Fatalf("invalid rows = %+v, want one at line 2", bad)
	}
	if len(txns) != 1 {
		t.Fatalf("valid rows = %d, want 1", len(txns))
	}
}

func TestParseBankCSVUnknownColumnDoesNotEchoHeader(t *testing.T) {
	in := "SECRET_TOKEN=abc,Amount\n1,2\n"
	_, _, err := actual.ParseBankCSV(strings.NewReader(in), actual.CSVMapping{DateCol: "Date", AmountCol: "Amount"})
	if err == nil || strings.Contains(err.Error(), "SECRET_TOKEN") {
		t.Fatalf("error = %v, want a column error without header contents", err)
	}
}

// A mirror left behind under a file id by an older pull must not shadow a
// newer pull keyed on the group id, and either id finds the newest copy.
func TestLocateMirrorPrefersNewestMatch(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	srv := fakeServer(t, budgetZip(t), nil, "")
	c := actual.New(srv.URL, 10*time.Second)
	res, err := actual.Pull(context.Background(), c, actual.PullOptions{Password: "hunter2", SyncID: "file-1"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-fix layout: an older copy under mirrors/file-1.
	oldDir, _ := actual.MirrorDir("file-1")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := *res.Manifest
	old.SyncID, old.PulledAt = "file-1", old.PulledAt.Add(-48*time.Hour)
	mj, _ := json.Marshal(old)
	_ = os.WriteFile(filepath.Join(oldDir, "pull.json"), mj, 0o600)
	_ = os.WriteFile(filepath.Join(oldDir, actual.DBFileName), []byte("stale"), 0o600)
	for _, id := range []string{"file-1", "group-1"} {
		path, m, found, err := actual.LocateMirror(id)
		if err != nil || !found || path != res.DBPath || m == nil || m.SyncID != "group-1" {
			t.Errorf("LocateMirror(%q) = %s %+v %v %v, want %s", id, path, m, found, err, res.DBPath)
		}
	}
	if path, m, found, err := actual.LocateMirror("other"); err != nil || found || m != nil || path != "" {
		t.Errorf("LocateMirror(other) = %q %+v %v %v, want not found", path, m, found, err)
	}
}
