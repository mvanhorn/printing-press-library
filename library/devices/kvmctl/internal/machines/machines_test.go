package machines

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInventoryValidationAndExactTargetBinding(t *testing.T) {
	inv := Inventory{Targets: []Target{{Name: "pve1", Port: 1, Enabled: true, OCRPatterns: []string{"pve1"}}}}
	if err := inv.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := inv.Resolve("pve"); err == nil {
		t.Fatal("expected exact-only target resolution")
	}
	got, err := inv.Resolve("pve1")
	if err != nil || got.Port != 1 {
		t.Fatalf("resolve: %#v %v", got, err)
	}
	inv.Targets[0].Port = 0
	if err := inv.Validate(); err == nil {
		t.Fatal("expected invalid port")
	}
}

func TestFileLockSerializesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	l1, err := NewDeviceLock(filepath.Join(dir, "device"))
	if err != nil {
		t.Fatal(err)
	}
	l2, err := NewDeviceLock(filepath.Join(dir, "device"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := l1.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l2.TryAcquire(); !errors.Is(err, ErrLocked) {
		t.Fatalf("got %v", err)
	}
	if err := l1.Release(); err != nil {
		t.Fatal(err)
	}
	if err := l2.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l2.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "device")); !os.IsNotExist(err) {
		t.Fatalf("lock file not cleaned: %v", err)
	}
}

func TestSelectVerifiesBeforeReturning(t *testing.T) {
	inv := DefaultInventory()
	events := []string{}
	s := Selector{Inventory: inv, LockFactory: func(string) (Locker, error) { return nopLocker{}, nil }, SendKey: func(ctx context.Context, key string, down bool) error { events = append(events, key); return nil }, Verify: func(context.Context, Target) error { return nil }}
	rec, err := s.Select(context.Background(), "pve2", SelectOptions{Rearm: false, Settle: time.Nanosecond, Hold: time.Nanosecond, Gap: time.Nanosecond, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != Verified || rec.Target.Name != "pve2" || len(events) != 8 {
		t.Fatalf("record=%#v events=%v", rec, events)
	}
}

func TestSelectTimeoutLeavesUnverified(t *testing.T) {
	s := Selector{Inventory: DefaultInventory(), LockFactory: func(string) (Locker, error) { return nopLocker{}, nil }, SendKey: func(ctx context.Context, key string, down bool) error { <-ctx.Done(); return ctx.Err() }, Verify: func(context.Context, Target) error { return nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	rec, err := s.Select(ctx, "pve1", SelectOptions{Rearm: false, Hold: time.Nanosecond, Gap: time.Nanosecond, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err == nil || rec.State == Verified {
		t.Fatalf("rec=%#v err=%v", rec, err)
	}
}

type nopLocker struct{}

func (nopLocker) Acquire(context.Context) error { return nil }
func (nopLocker) TryAcquire() error             { return nil }
func (nopLocker) Release() error                { return nil }
