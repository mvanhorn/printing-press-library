package tbprofile

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func touchBook(t *testing.T, path string, n int) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	card := fmt.Sprintf("c0c0c0c0-0000-4000-8000-%012d", n)
	if _, err := db.Exec(`INSERT INTO properties VALUES (?, 'PrimaryEmail', ?)`, card, fmt.Sprintf("new%d@example.com", n)); err != nil {
		t.Fatal(err)
	}
}

func withSnapshotSeam(t *testing.T, fn func(dbPath string)) *int {
	t.Helper()
	calls := 0
	origSeam, origBackoff := snapshotCopied, snapshotBackoff
	snapshotCopied = func(p string) { calls++; fn(p) }
	snapshotBackoff = 0
	t.Cleanup(func() { snapshotCopied, snapshotBackoff = origSeam, origBackoff })
	return &calls
}

func TestSnapshotRetriesWhenSourceChangesDuringCopy(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	path := filepath.Join(profile, "abook.sqlite")
	changed := false
	calls := withSnapshotSeam(t, func(p string) {
		if !changed {
			changed = true
			touchBook(t, p, 1)
		}
	})
	cards, err := ReadAddressBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if *calls != 2 || len(cards) != 3 {
		t.Fatalf("attempts = %d, cards = %d; want a retry that sees the new card", *calls, len(cards))
	}
}

func TestSnapshotErrorsWhenSourceNeverSettles(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	path := filepath.Join(profile, "abook.sqlite")
	n := 0
	calls := withSnapshotSeam(t, func(p string) { n++; touchBook(t, p, n) })
	if _, err := ReadAddressBook(path); err == nil {
		t.Fatal("a source that changes on every copy must not be read")
	}
	if *calls != snapshotAttempts {
		t.Fatalf("attempts = %d, want %d", *calls, snapshotAttempts)
	}
}
