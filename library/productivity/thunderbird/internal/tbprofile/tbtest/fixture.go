// Package tbtest builds the synthetic Thunderbird fixture profile for tests.
package tbtest

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ProfileRel is the fixture profile directory relative to the root.
const ProfileRel = "Profiles/abcd1234.default-release"

// Fixture card and event ids.
const (
	CardAlice = "a1b2c3d4-0000-4000-8000-000000000001"
	CardCarol = "a1b2c3d4-0000-4000-8000-000000000002"
	CardFrank = "a1b2c3d4-0000-4000-8000-000000000003"
	EventSync = "e1e1e1e1-0000-4000-8000-000000000001"
)

// TestdataRoot returns the checked-in fixture Thunderbird root.
func TestdataRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "root")
}

// Fixture copies the fixture root to a temp dir, creates abook.sqlite,
// history.sqlite and calendar-data/local.sqlite, and returns the root and
// profile directory.
func Fixture(t testing.TB) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Thunderbird")
	if err := os.CopyFS(root, os.DirFS(TestdataRoot())); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	profile := filepath.Join(root, filepath.FromSlash(ProfileRel))
	execAll(t, filepath.Join(profile, "abook.sqlite"),
		`CREATE TABLE properties (card TEXT, name TEXT, value TEXT)`,
		`INSERT INTO properties VALUES
			('`+CardAlice+`','DisplayName','Alice Example'),
			('`+CardAlice+`','FirstName','Alice'),
			('`+CardAlice+`','LastName','Example'),
			('`+CardAlice+`','NickName','Al'),
			('`+CardAlice+`','Company','Example Corp'),
			('`+CardAlice+`','PrimaryEmail','alice@example.com'),
			('`+CardAlice+`','SecondEmail','alice.alt@example.com'),
			('`+CardCarol+`','DisplayName','Carol Example'),
			('`+CardCarol+`','PrimaryEmail','CAROL@example.com')`,
	)
	execAll(t, filepath.Join(profile, "history.sqlite"),
		`CREATE TABLE properties (card TEXT, name TEXT, value TEXT)`,
		`INSERT INTO properties VALUES ('`+CardFrank+`','PrimaryEmail','frank@example.com')`,
	)
	start := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC).UnixMicro()
	end := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC).UnixMicro()
	later := time.Date(2025, 2, 1, 8, 0, 0, 0, time.UTC).UnixMicro()
	if err := os.MkdirAll(filepath.Join(profile, "calendar-data"), 0o750); err != nil {
		t.Fatal(err)
	}
	execAll(t, filepath.Join(profile, "calendar-data", "local.sqlite"),
		`CREATE TABLE cal_events (cal_id TEXT, id TEXT, title TEXT, event_start INTEGER, event_end INTEGER)`,
		`CREATE TABLE cal_properties (item_id TEXT, key TEXT, value BLOB, cal_id TEXT)`,
		`INSERT INTO cal_events VALUES ('cal-1','`+EventSync+`','Team sync',`+itoa(start)+`,`+itoa(end)+`)`,
		`INSERT INTO cal_events VALUES ('cal-1','e2','Dentist',`+itoa(later)+`,`+itoa(later)+`)`,
		`INSERT INTO cal_properties VALUES ('`+EventSync+`','LOCATION','Room 1','cal-1')`,
	)
	return root, profile
}

// ToCRLF rewrites a file with CRLF line endings.
func ToCRLF(t testing.TB, path string) {
	t.Helper()
	path = filepath.Clean(path)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	b = bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	if err := os.WriteFile(path, b, 0o600); err != nil { // #nosec G703 -- test helper; path is a file inside the t.TempDir fixture
		t.Fatal(err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func execAll(t testing.TB, path string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}
