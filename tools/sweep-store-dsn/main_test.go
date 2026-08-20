package main

import (
	"strings"
	"testing"
)

// A trimmed but faithful slice of a canonical published store.go: the false
// OpenReadOnly comment, the read-only open, and the read-write open.
const canonicalStore = `// with "file:". Without the prefix, "?mode=ro" is silently dropped and
// the connection opens read-write. Underscore-prefixed driver pragmas
// (_journal_mode, _busy_timeout, etc.) work either way; they're parsed
// out of the DSN by the driver before sqlite3_open_v2.
func OpenReadOnly(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
	if err != nil {
		return nil, fmt.Errorf("opening database (read-only): %w", err)
	}
}

func OpenWithContext(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
}
`

func TestRewrite_FixesCanonicalStore(t *testing.T) {
	out, changed := rewriteStoreDSN(canonicalStore)
	if !changed {
		t.Fatal("expected canonical store to be rewritten")
	}
	for _, want := range []string{
		`?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)`,   // ro: no journal_mode
		`dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)`, // rw: keeps journal_mode
		"modernc.org/sqlite does NOT recognize the",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rewritten store missing %q", want)
		}
	}
	// The read-only DSN must NOT set journal_mode (errors on a delete-mode DB
	// opened read-only).
	if strings.Contains(out, "mode=ro&_pragma=journal_mode") {
		t.Error("read-only DSN must not carry a journal_mode pragma")
	}
	// Stale fragments must be gone. These are DSN-context substrings only —
	// the corrected comment legitimately mentions the bare _journal_mode=WAL /
	// _busy_timeout=5000 tokens in prose, so we anchor on the surrounding ? / &
	// DSN punctuation instead.
	for _, bad := range []string{
		"?mode=ro&_journal_mode=WAL",        // old read-only DSN
		`?_journal_mode=WAL&_synchronous`,   // old read-write DSN
		"&_busy_timeout=5000&_foreign_keys", // old DSN body
		"work either way",                   // false comment
		"before sqlite3_open_v2",            // old comment tail
	} {
		if strings.Contains(out, bad) {
			t.Errorf("rewritten store still contains stale fragment %q", bad)
		}
	}
}

// Idempotency is a hard requirement for a sweep: a second pass must be a no-op.
func TestRewrite_Idempotent(t *testing.T) {
	once, changed := rewriteStoreDSN(canonicalStore)
	if !changed {
		t.Fatal("first pass should change content")
	}
	twice, changedAgain := rewriteStoreDSN(once)
	if changedAgain {
		t.Error("second pass changed content; rewrite is not idempotent")
	}
	if once != twice {
		t.Error("second pass produced different output")
	}
}

// Bespoke or already-fixed stores must be left untouched — the sweep only
// matches the exact canonical strings.
func TestRewrite_SkipsNonCanonical(t *testing.T) {
	cases := map[string]string{
		"already _pragma":   `db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")`,
		"bespoke wrong key": `dsn := dbPath + "?_journal=WAL&_foreign_keys=on"`,
		"bare open":         `db, err := sql.Open("sqlite", dbPath)`,
		"url.URL builder":   `"mode=ro&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY"`,
		"in-memory":         `canon, err := sql.Open("sqlite", ":memory:")`,
	}
	for name, content := range cases {
		out, changed := rewriteStoreDSN(content)
		if changed || out != content {
			t.Errorf("%s: expected no change, but content was rewritten", name)
		}
	}
}
