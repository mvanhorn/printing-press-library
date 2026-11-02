package voice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLiteralUnicodeQueryOnExistingMirror(t *testing.T) {
	m := historyFixture(t)
	if _, err := m.DB.Exec("UPDATE voice_messages SET text='unrelated'"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DB.Exec("UPDATE voice_messages SET text='appointment?! café Straße ΟΣ' WHERE id='m-1'"); err != nil {
		t.Fatal(err)
	}
	path := m.Path
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	// Read-only reopening exercises existing mirrors without a schema upgrade.
	ro, err := OpenMirror(path, "account-id", false)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	for _, query := range []string{"point", "?!", "CAFÉ", "STRASSE", "ος"} {
		rows, err := ro.Messages(context.Background(), Query{Text: query, Limit: 1})
		if err != nil || len(rows) != 1 || rows[0].ID != "m-1" {
			t.Fatalf("query %q: rows=%v error=%v", query, rows, err)
		}
	}
}

func TestMirrorRejectsSharedParentWithoutChangingIt(t *testing.T) {
	dir := privateTestDir(t)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "messages.sqlite3")
	if m, err := OpenMirror(path, "account-id", true); err == nil {
		m.Close()
		t.Fatal("shared parent accepted")
	}
	fi, err := os.Stat(dir)
	if err != nil || fi.Mode().Perm() != 0755 {
		t.Fatalf("existing directory changed: %v %v", fi, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("database created before rejecting parent: %v", err)
	}
	private := filepath.Join(privateTestDir(t), "new-private", "messages.sqlite3")
	m, err := OpenMirror(private, "account-id", true)
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
}

func privateTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
