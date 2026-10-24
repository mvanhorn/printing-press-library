// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// A local reader pointed at another tool's SQLite file refuses it before
// chmod-ing, migrating or switching it to WAL.
func TestLocalReadersRefuseAForeignDB(t *testing.T) {
	testenv.Isolate(t)
	path := filepath.Join(t.TempDir(), "other.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE notes (x TEXT)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = runS3B(t, path, "traction", "324684580", "--json")
	if ExitCode(err) != 2 {
		t.Fatalf("a foreign SQLite file: exit %d (%v), want 2", ExitCode(err), err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("the refused file changed: %v %v", info.Mode().Perm(), err)
	}
	if _, err := os.Stat(path + "-wal"); err == nil {
		t.Fatal("the refused file was switched to WAL")
	}
}

// A store the user made read-only is read as it is, not chmod-ed.
func TestLocalReadersReadAReadOnlyStore(t *testing.T) {
	testenv.Isolate(t)
	st, db := s3bNewStore(t)
	_ = st.Close()
	if err := os.Chmod(db, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(db, 0o644) })
	if _, _, err := runS3B(t, db, "changes", "--json"); err != nil {
		t.Fatalf("read-only store: %v", err)
	}
	if info, _ := os.Stat(db); info.Mode().Perm() != 0o444 {
		t.Fatalf("a read-only store was chmod-ed to %v", info.Mode().Perm())
	}
}
