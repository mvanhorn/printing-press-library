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
	stat := func(name string) (os.FileInfo, error) {
		if name == path+"-wal" && !interleaved {
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
