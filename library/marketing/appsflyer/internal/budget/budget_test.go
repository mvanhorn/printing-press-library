package budget

import (
	"path/filepath"
	"testing"
)

func TestNewDefaultLimit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPSFLYER_CONFIG_DIR", dir)
	tracker, err := New("", 0)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if tracker.Limit() != DefaultDailyLimit {
		t.Errorf("Limit = %d, want %d", tracker.Limit(), DefaultDailyLimit)
	}
}

func TestChargeAndRemaining(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.json")
	tracker, err := New(path, 5)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	if rem := tracker.Remaining(); rem != 5 {
		t.Fatalf("initial Remaining = %d, want 5", rem)
	}
	tracker.Charge(1)
	if rem := tracker.Remaining(); rem != 4 {
		t.Fatalf("after charge(1) Remaining = %d, want 4", rem)
	}
	tracker.Charge(3)
	if rem := tracker.Remaining(); rem != 1 {
		t.Fatalf("after charge(3) Remaining = %d, want 1", rem)
	}
	tracker.Charge(10)
	if rem := tracker.Remaining(); rem != 0 {
		t.Fatalf("Remaining should clamp at 0, got %d", rem)
	}
}

func TestPersistsAcrossNewWithinSameDay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.json")
	t1, err := New(path, 20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t1.Charge(3)
	t2, err := New(path, 20)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := t2.Snapshot().Used; got != 3 {
		t.Fatalf("after reopen Used = %d, want 3", got)
	}
}

func TestSnapshotIncludesDate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.json")
	tracker, _ := New(path, 20)
	s := tracker.Snapshot()
	if s.Date == "" {
		t.Fatal("Snapshot.Date is empty")
	}
}

func TestPathReturnsConfiguredPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "budget.json")
	tracker, _ := New(path, 20)
	if tracker.Path() != path {
		t.Fatalf("Path = %q, want %q", tracker.Path(), path)
	}
}
