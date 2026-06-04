// Command sweep-store-dsn retrofits the SQLite DSN pragma syntax in every
// published CLI's internal/store/store.go.
//
// Background: the store template historically opened modernc.org/sqlite with
// mattn/go-sqlite3-style DSN params (?_journal_mode=WAL&_busy_timeout=5000&…).
// modernc.org/sqlite recognizes only the _pragma=name(value) form, so those
// params were parsed as unknown query keys and silently dropped — every
// published store ran in default delete-journal mode with busy_timeout=0, and
// a read concurrent with a write failed immediately with SQLITE_BUSY. The
// generator template was fixed upstream in cli-printing-press; this tool
// retrofits already-published entries so they match fresh prints.
//
// Scope is deliberately narrow. It replaces only the two exact canonical DSN
// string literals and the one false comment block that fresh prints emitted.
// Any store whose open is bespoke (variable-built DSN, url.URL builder, a
// pragma-less bare open, an external-DB snapshot reader, or one already on the
// _pragma= form) does not match and is left untouched — those need a reprint,
// not a mechanical edit. Idempotent: the replacements target the old syntax,
// so a second run produces zero diff.
//
// The read-only DSN intentionally carries no journal_mode pragma. Journal mode
// is a property of the database file (set by the read-write open), not the
// connection; PRAGMA journal_mode=WAL on a read-only handle to a DB still in
// the default delete mode errors with "attempt to write a readonly database".
//
// Run from the repo root (GOPATH mode, no go.mod):
//
//	SWEEP_LIBRARY_ROOT=library GO111MODULE=off go run ./tools/sweep-store-dsn
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The exact read-write DSN expression fresh prints emitted before the fix, and
// the corrected form. These are Go source substrings (string-concatenation
// expressions), matched verbatim.
const (
	oldRW = `dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456"`
	newRW = `dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)"`

	// The read-only form drops journal_mode (see package comment).
	oldRO = `"file:"+dbPath+"?mode=ro&_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456"`
	newRO = `"file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)"`
)

// The false comment block in OpenReadOnly, and its correction. Both are
// matched as a verbatim multi-line block so a store whose comment drifted does
// not get a partial edit; its DSN is still fixed by the literal replacements
// above.
const (
	oldComment = `// the connection opens read-write. Underscore-prefixed driver pragmas
// (_journal_mode, _busy_timeout, etc.) work either way; they're parsed
// out of the DSN by the driver before sqlite3_open_v2.`
	newComment = `// the connection opens read-write. Pragmas use the driver's _pragma=
// name(value) syntax — modernc.org/sqlite does NOT recognize the
// mattn/go-sqlite3 _journal_mode=WAL / _busy_timeout=5000 form and drops
// those keys silently, so the busy_timeout below is what keeps a read
// concurrent with a writer from failing immediately with SQLITE_BUSY.`
)

// rewriteStoreDSN applies the three verbatim replacements. It returns the new
// content and whether anything changed. Pure (no I/O) so tests can drive it
// directly and assert idempotency.
func rewriteStoreDSN(content string) (string, bool) {
	out := content
	out = strings.ReplaceAll(out, oldRW, newRW)
	out = strings.ReplaceAll(out, oldRO, newRO)
	out = strings.ReplaceAll(out, oldComment, newComment)
	return out, out != content
}

func libraryRoot() string {
	if r := os.Getenv("SWEEP_LIBRARY_ROOT"); r != "" {
		return r
	}
	return "library"
}

func main() {
	root := libraryRoot()
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "library root %q not found (run from repo root, or set SWEEP_LIBRARY_ROOT)\n", root)
		os.Exit(1)
	}

	var changed, scanned int
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(filepath.ToSlash(path), "/internal/store/store.go") {
			return nil
		}
		scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out, didChange := rewriteStoreDSN(string(data))
		if !didChange {
			return nil
		}
		// 0o644: generated Go sources are mode 0644; these files already
		// exist so os.WriteFile keeps their on-disk perms regardless, but a
		// concrete sensible mode avoids depending on that (d.Type().Perm()
		// is 0o000 for a regular file and would create world-inaccessible
		// files if a target ever didn't pre-exist).
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return err
		}
		changed++
		fmt.Printf("fixed %s\n", path)
		return nil
	})
	if walkErr != nil {
		fmt.Fprintf(os.Stderr, "sweep failed: %v\n", walkErr)
		os.Exit(1)
	}

	fmt.Printf("\nscanned %d store.go file(s); rewrote %d\n", scanned, changed)
}
