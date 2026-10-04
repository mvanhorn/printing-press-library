package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnowJapanReaderRejectsOpenWALAndRecovers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.DB().Exec("PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := writer.UpsertBatch("resorts", []json.RawMessage{snapshot("a", "detail-v1", "old", 100)}); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenSnowJapanReadOnlyContext(context.Background(), path); err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("open writer accepted: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenSnowJapanReadOnlyContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSnowJapanGuardRejectsHardLinksBeforeWriterOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	alias := path + "-alias"
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, alias} {
		if _, err := OpenSnowJapanWritableContext(context.Background(), p); err == nil || !strings.Contains(err.Error(), "hard-link") {
			t.Fatalf("ambiguous writer accepted: %v", err)
		}
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSnowJapanCanonicalSymlinkCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	alias := path + "-link"
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	writer, err := OpenSnowJapanWritableContext(context.Background(), alias)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if writer.Path() != canonical {
		t.Fatalf("writer uses alias WAL: %s", writer.Path())
	}
	raw := snapshot("a", "catalog-v1", "captured", 300)
	if _, _, err := writer.UpsertBatch("resorts", []json.RawMessage{raw}); err != nil {
		t.Fatal(err)
	}
	if err := writer.CaptureSnowJapan(context.Background(), []json.RawMessage{raw}, true); err != nil {
		t.Fatal(err)
	}
	if err := writer.SaveSyncState("resorts", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenSnowJapanReadOnlyContext(context.Background(), alias)
	if err != nil {
		t.Fatal(err)
	}
	facts, complete, err := reader.SnowJapanCatalog(context.Background())
	if err != nil || !complete || len(facts) != 1 {
		t.Fatalf("capture lost: %s complete=%v error=%v", facts, complete, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSnowJapanReaderPinsBeforeLazySwapRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facts.db")
	foreign := filepath.Join(dir, "foreign.db")
	for _, entry := range []struct {
		path string
		peak int
	}{{path, 100}, {foreign, 3000}} {
		writer, err := Open(entry.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := writer.UpsertBatch("resorts", []json.RawMessage{snapshot("a", "detail-v1", "at", entry.peak)}); err != nil {
			t.Fatal(err)
		}
		writer.Close()
	}
	reader, err := OpenSnowJapanReadOnlyContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	saved := path + "-saved"
	if err := os.Rename(path, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(foreign, path); err != nil {
		t.Fatal(err)
	}
	var peak int
	if err := reader.DB().QueryRow("SELECT json_extract(data,'$.peak_m') FROM resources WHERE resource_type='resorts'").Scan(&peak); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, foreign); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, path); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if peak != 100 {
		t.Fatalf("lazy connection bound foreign facts: %d", peak)
	}
}

func TestSnowJapanWriterRejectsRetargetBeforeSave(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.db")
	other := filepath.Join(dir, "other.db")
	for _, path := range []string{first, other} {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		s.Close()
	}
	alias := filepath.Join(dir, "alias.db")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	writer, err := OpenSnowJapanWritableContext(context.Background(), alias)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, alias); err != nil {
		t.Fatal(err)
	}
	if _, _, err := writer.UpsertBatch("resorts", []json.RawMessage{snapshot("a", "detail-v1", "at", 3000)}); err == nil {
		t.Fatal("retargeted capture reported success")
	}
	if err := writer.Close(); err == nil {
		t.Fatal("retargeted close reported success")
	}
}
