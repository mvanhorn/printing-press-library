// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSavedReadGuardRejectsActiveWALAndAllowsCompletedSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	writer, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := examplePlace(Restaurant, "300739")
	p.Name = "first"
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), writer.DB(), p); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path + "-wal")
	if err != nil || info.Size() == 0 {
		t.Fatalf("writer did not leave active WAL: %v", err)
	}
	_, err = BeginSavedRead(path)
	var visibility *CacheVisibilityError
	if !errors.As(err, &visibility) {
		t.Fatalf("active writer was not explicit: %v", err)
	}
	writer.Close()
	guard, err := BeginSavedRead(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := Snapshot(context.Background(), reader.DB(), Selection{Restaurant, "300739"}, 0)
	reader.Close()
	if err != nil || !ok || got.Name != "first" {
		t.Fatalf("completed save invisible: %#v %v", got, err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	writer, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "second"
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), writer.DB(), p); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	if err = guard.Check(); !errors.As(err, &visibility) {
		t.Fatalf("checkpoint completion race was not detected: %v", err)
	}
}
func TestSavedReadGuardRejectsFileReplacementAndUntruncatedWAL(t *testing.T) {
	for _, name := range []string{"replacement", "wal"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cache.db")
			if err := os.WriteFile(path, []byte("initial"), 0600); err != nil {
				t.Fatal(err)
			}
			guard, err := BeginSavedRead(path)
			if err != nil {
				t.Fatal(err)
			}
			if name == "replacement" {
				if err = os.WriteFile(path+".new", []byte("new image"), 0600); err != nil {
					t.Fatal(err)
				}
				err = os.Rename(path+".new", path)
			} else {
				err = os.WriteFile(path+"-wal", []byte("untruncated WAL"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			var visibility *CacheVisibilityError
			if err = guard.Check(); !errors.As(err, &visibility) {
				t.Fatalf("%s accepted: %v", name, err)
			}
		})
	}
}

func TestSavedReadGuardRejectsCheckpointBetweenMainAndWALStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	if err := os.WriteFile(path, []byte("old image"), 0600); err != nil {
		t.Fatal(err)
	}
	guard, err := BeginSavedRead(path)
	if err != nil {
		t.Fatal(err)
	}
	interleaved := false
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	stat := func(name string) (os.FileInfo, error) {
		if name == canonical+"-wal" && !interleaved {
			interleaved = true
			if err := os.WriteFile(path, []byte("new checkpointed image"), 0600); err != nil {
				return nil, err
			}
		}
		return cacheFileInfo(name)
	}
	var visibility *CacheVisibilityError
	if err = guard.checkWithStat(stat); !errors.As(err, &visibility) {
		t.Fatalf("stat-interleave checkpoint was accepted: %v", err)
	}
	if !interleaved {
		t.Fatal("interleave was not exercised")
	}
}

func TestSavedReadGuardResolvesAliasesAndRejectsAmbiguousHardLinks(t *testing.T) {
	dir := t.TempDir()
	path, alias := filepath.Join(dir, "cache.db"), filepath.Join(dir, "alias.db")
	writer, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := examplePlace(Restaurant, "300739")
	p.Name = "checkpointed"
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), writer.DB(), p); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	if err = os.Symlink(path, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := BeginSavedRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	writer, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "committed in real WAL"
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), writer.DB(), p); err != nil {
		t.Fatal(err)
	}
	var visibility *CacheVisibilityError
	if _, err = BeginSavedRead(alias); !errors.As(err, &visibility) {
		t.Fatalf("symlink hid real WAL: %v", err)
	}
	if err = guard.Check(); !errors.As(err, &visibility) {
		t.Fatalf("existing symlink guard accepted real WAL: %v", err)
	}
	writer.Close()
	guard, err = BeginSavedRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.OpenReadOnly(alias)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := Snapshot(context.Background(), reader.DB(), Selection{Restaurant, "300739"}, 0)
	reader.Close()
	if err != nil || !ok || got.Name != p.Name {
		t.Fatalf("closed writer alias read lost current facts: %+v %v", got, err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(dir, "hard.db")
	if err = os.Link(path, hard); err != nil {
		t.Skipf("hard link unavailable: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, known := cacheLinkCount(info); !known {
		t.Skip("host does not expose hard-link count")
	}
	for _, selected := range []string{path, alias, hard} {
		if _, err = BeginSavedRead(selected); !errors.As(err, &visibility) {
			t.Fatalf("multiple links accepted at %s: %v", selected, err)
		}
	}
}

func TestSavedReadGuardRejectsAliasRetargetingAndNewHardLink(t *testing.T) {
	dir := t.TempDir()
	first, second, alias := filepath.Join(dir, "first.db"), filepath.Join(dir, "second.db"), filepath.Join(dir, "alias.db")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("same image"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(first, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := BeginSavedRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	var visibility *CacheVisibilityError
	if err = guard.Check(); !errors.As(err, &visibility) {
		t.Fatalf("retargeted alias accepted: %v", err)
	}
	guard, err = BeginSavedRead(first)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Link(first, filepath.Join(dir, "hard.db")); err != nil {
		t.Skipf("hard link unavailable: %v", err)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, known := cacheLinkCount(info); known {
		if err = guard.Check(); !errors.As(err, &visibility) {
			t.Fatalf("hard link created during read accepted: %v", err)
		}
	}
}

func TestSavedWriteGuardUsesCanonicalWALAndRejectsHardLinks(t *testing.T) {
	dir := t.TempDir()
	path, alias := filepath.Join(dir, "cache.db"), filepath.Join(dir, "alias.db")
	first, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := examplePlace(Restaurant, "300739")
	p.Name = "first source observation"
	p.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), first.DB(), p); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(path, alias); err != nil {
		first.Close()
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := BeginSavedWrite(alias)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || guard.Path() != canonical {
		t.Fatalf("writer path=%s canonical=%s err=%v", guard.Path(), canonical, err)
	}
	second, err := store.Open(guard.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	p.Name = "new source observation"
	p.ObservedAt = time.Now().Add(time.Millisecond).UTC().Format(time.RFC3339Nano)
	if err = SaveSnapshot(context.Background(), second.DB(), p); err != nil {
		t.Fatal(err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	second.Close()
	first.Close()
	if _, err = os.Stat(alias + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("alias WAL was created: %v", err)
	}
	reader, err := store.OpenReadOnly(alias)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := Snapshot(context.Background(), reader.DB(), Selection{Restaurant, "300739"}, 0)
	reader.Close()
	if err != nil || !ok || got.Name != p.Name || got.ObservedAt != p.ObservedAt {
		t.Fatalf("new observation was lost when canonical writer closed: %+v %v", got, err)
	}
	hard := filepath.Join(dir, "hard.db")
	if err = os.Link(path, hard); err != nil {
		t.Skipf("hard link unavailable: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, known := cacheLinkCount(info); !known {
		t.Skip("host does not expose hard-link count")
	}
	var visibility *CacheVisibilityError
	for _, selected := range []string{path, alias, hard} {
		if _, err = BeginSavedWrite(selected); !errors.As(err, &visibility) {
			t.Fatalf("hard-linked writer accepted at %s: %v", selected, err)
		}
	}
}

func TestSavedWriteGuardAllowsNewCacheAndDetectsAliasRetarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new", "data", "cache.db")
	guard, err := BeginSavedWrite(path)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := store.Open(guard.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err = guard.Check(); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	alias := filepath.Join(dir, "alias.db")
	if err = os.Symlink(path, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err = BeginSavedWrite(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.db")
	if err = os.WriteFile(other, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(other, alias); err != nil {
		t.Fatal(err)
	}
	var visibility *CacheVisibilityError
	if err = guard.Check(); !errors.As(err, &visibility) {
		t.Fatalf("writer alias retarget accepted: %v", err)
	}
}
