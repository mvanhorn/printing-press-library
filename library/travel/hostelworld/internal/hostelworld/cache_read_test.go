package hostelworld

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func cacheFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cache.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("planning_snapshot", "base", json.RawMessage(`{"name":"baseline"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestCacheGuardResolvesAliasesAndRejectsOpenJournals(t *testing.T) {
	real := cacheFixture(t)
	alias := filepath.Join(t.TempDir(), "alias.db")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	guard, err := BeginCacheRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if guard.Path() != physical {
		t.Fatal("selected alias did not resolve to physical file")
	}
	if err := guard.Check(); err != nil {
		t.Fatal(err)
	}
	writer, err := store.OpenWithContext(context.Background(), real)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Upsert("planning_snapshot", "new", json.RawMessage(`{"name":"committed"}`)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{real, alias} {
		if _, err := BeginCacheRead(path); err == nil {
			t.Fatal("open WAL hidden by", path)
		}
		if _, err := BeginCacheWrite(path); err != nil {
			t.Fatal("ordinary canonical SQLite writer blocked", path, err)
		}
	}
	if err := guard.Check(); err == nil {
		t.Fatal("WAL creation during complete read was invisible")
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := guard.Check(); err == nil {
		t.Fatal("checkpoint mutation during read was invisible")
	}
	next, err := BeginCacheRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Check(); err != nil {
		t.Fatal(err)
	}
	// Rollback journals are also unsafe; no reader or writer may call them empty.
	if err := os.WriteFile(real+"-journal", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := BeginCacheRead(alias); err == nil {
		t.Fatal("rollback journal ignored")
	}

}
func TestCacheGuardRejectsHardLinksMissingTargetsAndReplacements(t *testing.T) {
	real := cacheFixture(t)
	alias := filepath.Join(t.TempDir(), "linked.db")
	if err := os.Link(real, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{real, alias} {
		if _, err := BeginCacheRead(path); err == nil {
			t.Fatal("hard-linked read accepted")
		}
		if _, err := BeginCacheWrite(path); err == nil {
			t.Fatal("hard-linked writer accepted")
		}
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	guard, err := BeginCacheRead(real)
	if err != nil {
		t.Fatal(err)
	}
	writeGuard, err := BeginCacheWrite(real)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(real, real+"-old"); err != nil {
		t.Fatal(err)
	}
	replacement := cacheFixture(t)
	data, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := guard.Check(); err == nil {
		t.Fatal("selected read path replacement accepted")
	}
	if err := writeGuard.BindWriter(); err == nil {
		t.Fatal("selected writer path replacement accepted")
	}
	broken := filepath.Join(t.TempDir(), "broken.db")
	if err := os.Symlink(filepath.Join(t.TempDir(), "missing.db"), broken); err != nil {
		t.Fatal(err)
	}
	if _, err := BeginCacheRead(broken); err == nil {
		t.Fatal("broken alias reported empty")
	}
	if _, err := BeginCacheWrite(broken); err == nil {
		t.Fatal("broken alias silently created")
	}
}
func TestCacheGuardChecksSymlinkRetargetAndNewFileIdentity(t *testing.T) {
	first, second := cacheFixture(t), cacheFixture(t)
	alias := filepath.Join(t.TempDir(), "alias.db")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	guard, err := BeginCacheRead(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	if err := guard.Check(); err == nil {
		t.Fatal("selected symlink retarget accepted")
	}
	missing := filepath.Join(t.TempDir(), "missing", "data.db")
	create, err := BeginCacheWrite(missing)
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(context.Background(), create.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := create.BindWriter(); err != nil {
		t.Fatal(err)
	}
	if err := create.CheckWriter(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := create.CheckAfterWriterClose(); err != nil {
		t.Fatal(err)
	}
}
