// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
)

func readerFixture(t *testing.T, path, name string) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := examplePlace(Restaurant, "300739")
	p.Name, p.ObservedAt = name, time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), db.DB(), p); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSavedReaderSnapshotCannotQuerySubstitutedPath(t *testing.T) {
	dir := t.TempDir()
	path, replacement, backup := filepath.Join(dir, "cache.db"), filepath.Join(dir, "replacement.db"), filepath.Join(dir, "backup.db")
	readerFixture(t, path, "verified original row")
	readerFixture(t, replacement, "substituted row")
	guard, err := BeginSavedRead(path)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	if guard.Path() == guard.canonical {
		t.Fatal("SQLite is still opening the public pathname")
	}
	db, err := store.OpenReadOnly(guard.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = os.Rename(path, backup); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Snapshot(context.Background(), db.DB(), Selection{Restaurant, "300739"}, 0)
	if err != nil || !ok || got.Name != "verified original row" {
		t.Fatalf("SQL read substituted pathname rows: %+v %v", got, err)
	}
	if err = os.Rename(path, replacement); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	// The pathname is restored before the final stat check, yet actual SQL
	// remained bound to the verified original descriptor's private image.
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	snapshot := guard.Path()
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if err = guard.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(snapshot); !os.IsNotExist(err) {
		t.Fatalf("private reader snapshot retained: %v", err)
	}
}

func TestSavedReaderRejectsSubstitutedDescriptorBeforeCopy(t *testing.T) {
	dir := t.TempDir()
	path, replacement := filepath.Join(dir, "cache.db"), filepath.Join(dir, "replacement.db")
	readerFixture(t, path, "original")
	readerFixture(t, replacement, "substituted")
	guard, err := savedReadState(path, cacheFileInfo)
	if err != nil {
		t.Fatal(err)
	}
	err = guard.snapshotWithOpen(context.Background(), func(string) (*os.File, error) { return os.Open(replacement) })
	var visibility *CacheVisibilityError
	if !errors.As(err, &visibility) {
		t.Fatalf("different opened inode accepted: %v", err)
	}
	if guard.privateDir != "" {
		t.Fatal("failed binding retained snapshot state")
	}
}

func TestSavedReaderSnapshotCancellationAndSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	if err := os.WriteFile(path, []byte("small"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BeginSavedReadContext(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot accepted: %v", err)
	}
	if err := os.Truncate(path, maxSavedReadSnapshotBytes+1); err != nil {
		t.Fatal(err)
	}
	var visibility *CacheVisibilityError
	if _, err := BeginSavedRead(path); !errors.As(err, &visibility) {
		t.Fatalf("oversized snapshot accepted: %v", err)
	}
}

func TestCacheReservedURIBytesRejectBeforeOpeningDifferentCache(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "cache.db")
	readerFixture(t, plain, "unrelated plain cache must survive")
	before, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cache.db?mode=rwc", "cache.db#fragment", "cache%2edb"} {
		path := filepath.Join(dir, name)
		var visibility *CacheVisibilityError
		if _, err = BeginSavedWrite(path); !errors.As(err, &visibility) {
			t.Fatalf("URI-shaped writer path accepted: %s %v", name, err)
		}
		if _, err = BeginSavedRead(path); !errors.As(err, &visibility) {
			t.Fatalf("URI-shaped reader path accepted: %s %v", name, err)
		}
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rejected path created: %v", err)
		}
	}
	after, err := os.ReadFile(plain)
	if err != nil || string(before) != string(after) {
		t.Fatal("plain cache changed while rejecting URI paths")
	}
	unsafe := filepath.Join(dir, "existing?.db")
	if err = os.WriteFile(unsafe, []byte("must remain unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "clean-alias.db")
	if err = os.Symlink(unsafe, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var visibility *CacheVisibilityError
	if _, err = BeginSavedWrite(alias); !errors.As(err, &visibility) {
		t.Fatalf("resolved reserved path accepted: %v", err)
	}
}

func TestSavedReaderRejectsURIBytesInTemporaryDestination(t *testing.T) {
	dir := t.TempDir()
	path, trap := filepath.Join(dir, "original.db"), filepath.Join(dir, "temp-parent.db")
	readerFixture(t, path, "verified original row")
	readerFixture(t, trap, "substituted temporary URI row")
	before, err := os.ReadFile(trap)
	if err != nil {
		t.Fatal(err)
	}
	unsafe := trap + "?x=1"
	if err = os.Mkdir(unsafe, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", unsafe)
	var visibility *CacheVisibilityError
	if _, err = BeginSavedRead(path); !errors.As(err, &visibility) {
		t.Fatalf("URI-shaped temporary destination accepted: %v", err)
	}
	entries, err := os.ReadDir(unsafe)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed reader retained temporary image: %v %v", entries, err)
	}
	after, err := os.ReadFile(trap)
	if err != nil || string(before) != string(after) {
		t.Fatal("URI trap database changed")
	}
	clean := filepath.Join(dir, "clean-temp-alias")
	if err = os.Symlink(unsafe, clean); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Setenv("TMPDIR", clean)
	if _, err = BeginSavedRead(path); !errors.As(err, &visibility) {
		t.Fatalf("canonical temporary destination URI bytes accepted: %v", err)
	}
}
