// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// ParseAddress
// ---------------------------------------------------------------------

func TestParseAddress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    Address
		wantErr bool
	}{
		{"owner/repo", "mjwoon/AI-readings", Address{Owner: "mjwoon", Repo: "AI-readings"}, false},
		{"owner/repo/sub/path", "mjwoon/AI-readings/books/ml", Address{Owner: "mjwoon", Repo: "AI-readings", Path: "books/ml"}, false},
		{"owner/repo#ref", "mjwoon/AI-readings#main", Address{Owner: "mjwoon", Repo: "AI-readings", Ref: "main"}, false},
		{"owner/repo/path#ref", "mjwoon/AI-readings/books#v1", Address{Owner: "mjwoon", Repo: "AI-readings", Path: "books", Ref: "v1"}, false},
		{"repo URL", "https://github.com/octocat/Hello-World", Address{Owner: "octocat", Repo: "Hello-World"}, false},
		{"tree URL", "https://github.com/mjwoon/AI-readings/tree/main/books", Address{Owner: "mjwoon", Repo: "AI-readings", Ref: "main", Path: "books"}, false},
		{"blob URL", "https://github.com/octocat/Hello-World/blob/main/README.md", Address{Owner: "octocat", Repo: "Hello-World", Ref: "main", Path: "README.md"}, false},
		{"tree URL nested path", "https://github.com/mjwoon/AI-readings/tree/main/books/machine%20learning", Address{Owner: "mjwoon", Repo: "AI-readings", Ref: "main", Path: "books/machine learning"}, false},
		{"trailing slash trimmed", "mjwoon/AI-readings/books/", Address{Owner: "mjwoon", Repo: "AI-readings", Path: "books"}, false},
		{"URL with explicit ref suffix override", "https://github.com/mjwoon/AI-readings/tree/main/books#v2", Address{Owner: "mjwoon", Repo: "AI-readings", Ref: "v2", Path: "books"}, false},
		{"repo URL with ref suffix", "https://github.com/octocat/Hello-World#develop", Address{Owner: "octocat", Repo: "Hello-World", Ref: "develop"}, false},
		{"empty", "", Address{}, true},
		{"only hash", "#ref", Address{}, true},
		{"missing repo", "owner-only", Address{}, true},
		{"missing owner shorthand", "/repo", Address{}, true},
		{"unsupported host", "https://gitlab.com/owner/repo", Address{}, true},
		{"tree URL missing ref", "https://github.com/owner/repo/tree", Address{}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAddress(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAddress(%q) = %+v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddress(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseAddress(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// EscapePath
// ---------------------------------------------------------------------

func TestEscapePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"already safe", "books/foo.pdf", "books/foo.pdf"},
		{"spaces", "books/machine learning/foo bar.pdf", "books/machine%20learning/foo%20bar.pdf"},
		{"unicode", "books/café/naïve.txt", "books/caf%C3%A9/na%C3%AFve.txt"},
		{"parens escaped like other reserved chars", "books/foo (draft).pdf", "books/foo%20%28draft%29.pdf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EscapePath(tc.input)
			if got != tc.want {
				t.Fatalf("EscapePath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// GitBlobSHA / GitBlobSHAFile
// ---------------------------------------------------------------------

func TestGitBlobSHA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"empty file", "", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"hello newline", "hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := GitBlobSHA(strings.NewReader(tc.content), int64(len(tc.content)))
			if err != nil {
				t.Fatalf("GitBlobSHA(%q) unexpected error: %v", tc.content, err)
			}
			if got != tc.want {
				t.Fatalf("GitBlobSHA(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}

func TestGitBlobSHAFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := dir + "/hello.txt"
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	got, err := GitBlobSHAFile(path)
	if err != nil {
		t.Fatalf("GitBlobSHAFile: unexpected error: %v", err)
	}
	want := "ce013625030ba8dba906f756967f9e9ca394464a"
	if got != want {
		t.Fatalf("GitBlobSHAFile = %q, want %q", got, want)
	}

	if _, err := GitBlobSHAFile(dir + "/does-not-exist.txt"); err == nil {
		t.Fatal("GitBlobSHAFile on missing file: want error, got nil")
	}
}

// ---------------------------------------------------------------------
// IsLFSPointer
// ---------------------------------------------------------------------

func TestIsLFSPointer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		head []byte
		want bool
	}{
		{"real pointer", []byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 123\n"), true},
		{"ordinary text", []byte("hello world\n"), false},
		{"empty", []byte(""), false},
		{"binary garbage", []byte{0x00, 0x01, 0xFF, 0xFE}, false},
		{"prefix substring elsewhere", []byte("not version https://git-lfs.github.com/spec/v1"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsLFSPointer(tc.head)
			if got != tc.want {
				t.Fatalf("IsLFSPointer(%q) = %v, want %v", tc.head, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// MatchGlobs
// ---------------------------------------------------------------------

func TestMatchGlobs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		relPath  string
		includes []string
		excludes []string
		want     bool
	}{
		{"no filters matches all", "books/foo.pdf", nil, nil, true},
		{"include only match", "books/foo.pdf", []string{"*.pdf"}, nil, true},
		{"include only no match", "books/foo.epub", []string{"*.pdf"}, nil, false},
		{"include matches full path pattern", "books/foo.pdf", []string{"books/*.pdf"}, nil, true},
		{"exclude only wins", "books/foo.pdf", nil, []string{"*.pdf"}, false},
		{"include and exclude both set, exclude wins", "books/foo.pdf", []string{"*.pdf"}, []string{"books/*"}, false},
		{"include and exclude both set, include passes", "notes/foo.md", []string{"*.md"}, []string{"books/*"}, true},
		{"basename match for nested path", "books/ml/deep/notes.md", []string{"*.md"}, nil, true},
		{"invalid include pattern is silent non-match", "books/foo.pdf", []string{"[bad"}, nil, false},
		{"invalid exclude pattern is silent non-exclude", "books/foo.pdf", nil, []string{"[bad"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := MatchGlobs(tc.relPath, tc.includes, tc.excludes)
			if got != tc.want {
				t.Fatalf("MatchGlobs(%q, %v, %v) = %v, want %v", tc.relPath, tc.includes, tc.excludes, got, tc.want)
			}
		})
	}
}

func TestInvalidGlobs(t *testing.T) {
	t.Parallel()
	got := InvalidGlobs([]string{"*.pdf", "[bad", "", "*.md"})
	if len(got) != 1 || got[0] != "[bad" {
		t.Fatalf("InvalidGlobs = %v, want [\"[bad\"]", got)
	}
}

// ---------------------------------------------------------------------
// SafeRelPath
// ---------------------------------------------------------------------

func TestSafeRelPath(t *testing.T) {
	t.Parallel()

	type safeRelCase struct {
		name    string
		input   string
		want    string
		wantErr bool
	}
	cases := []safeRelCase{
		{"simple", "books/foo.pdf", "books/foo.pdf", false},
		{"leading dot-slash cleaned", "./books/foo.pdf", "books/foo.pdf", false},
		{"empty", "", "", true},
		{"absolute path rejected", "/etc/passwd", "", true},
		{"traversal rejected", "../../etc/passwd", "", true},
		{"internal traversal that stays inside is safe", "books/sub/../foo.pdf", "books/foo.pdf", false},
		{"traversal that escapes root", "books/../../etc/passwd", "", true},
		{"bare dot rejected", ".", "", true},
	}
	// The ':' rejection is Windows-only: on Unix hosts ':' is an ordinary
	// filename character (timestamped names like "12:30:00.log" are
	// legitimate repo content); on Windows it is drive-relative / NTFS
	// alternate-data-stream syntax.
	for _, cc := range []struct{ name, input string }{
		{"windows drive-relative", "C:evil"},
		{"ntfs alternate data stream", "foo:bar"},
		{"colon in nested segment", "books/a:b/x.pdf"},
	} {
		if runtime.GOOS == "windows" {
			cases = append(cases, safeRelCase{cc.name + " rejected on windows", cc.input, "", true})
		} else {
			cases = append(cases, safeRelCase{cc.name + " allowed on unix", cc.input, cc.input, false})
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := SafeRelPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("SafeRelPath(%q) = %q, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("SafeRelPath(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("SafeRelPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// TreeFile.RelTo
// ---------------------------------------------------------------------

func TestTreeFileRelTo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		file     TreeFile
		basePath string
		want     string
	}{
		{"whole repo (empty base)", TreeFile{Path: "books/foo.pdf"}, "", "books/foo.pdf"},
		{"scoped to directory", TreeFile{Path: "books/ml/foo.pdf"}, "books", "ml/foo.pdf"},
		{"single-file target", TreeFile{Path: "books/foo.pdf"}, "books/foo.pdf", "foo.pdf"},
		{"prefix without segment boundary is not under base", TreeFile{Path: "booksmore/x"}, "books", "booksmore/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.file.RelTo(tc.basePath)
			if got != tc.want {
				t.Fatalf("RelTo(%q) = %q, want %q", tc.basePath, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// WalkTree — fake API
// ---------------------------------------------------------------------

// fakeAPI implements the ghfetch.API interface against a fixed routing
// table of path -> JSON response, for exercising WalkTree without a live
// server.
type fakeAPI struct {
	responses map[string]string
	calls     []string
}

func (f *fakeAPI) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	key := path
	if params["recursive"] == "1" {
		key = path + "?recursive=1"
	}
	f.calls = append(f.calls, key)
	resp, ok := f.responses[key]
	if !ok {
		return nil, fmt.Errorf("fakeAPI: no response registered for %s", key)
	}
	return json.RawMessage(resp), nil
}

func TestWalkTreeRecursiveSuccess(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{responses: map[string]string{
		"/repos/o/r": `{"default_branch":"main"}`,
		"/repos/o/r/git/trees/main?recursive=1": `{
			"sha": "root",
			"truncated": false,
			"tree": [
				{"path": "books/foo.pdf", "mode": "100644", "type": "blob", "sha": "sha1", "size": 100},
				{"path": "books/bar.pdf", "mode": "100755", "type": "blob", "sha": "sha2", "size": 200},
				{"path": "books/legacy.txt", "mode": "100664", "type": "blob", "sha": "sha7", "size": 7},
				{"path": "books/link", "mode": "120000", "type": "blob", "sha": "sha3", "size": 10},
				{"path": "books/submod", "mode": "160000", "type": "commit", "sha": "sha4", "size": 0},
				{"path": "other/ignored.txt", "mode": "100644", "type": "blob", "sha": "sha5", "size": 5},
				{"path": "books", "mode": "040000", "type": "tree", "sha": "sha6", "size": 0}
			]
		}`,
	}}

	result, err := WalkTree(context.Background(), api, Address{Owner: "o", Repo: "r", Path: "books"})
	if err != nil {
		t.Fatalf("WalkTree unexpected error: %v", err)
	}
	if result.Ref != "main" {
		t.Fatalf("Ref = %q, want main", result.Ref)
	}
	if result.Truncated {
		t.Fatal("Truncated = true, want false")
	}
	// 3 files: the two canonical-mode blobs plus the legacy 100664 blob —
	// any blob mode other than 120000 (symlink) is downloadable.
	if len(result.Files) != 3 {
		t.Fatalf("len(Files) = %d, want 3 (got %+v)", len(result.Files), result.Files)
	}
	if len(result.SkippedSymlinks) != 1 || result.SkippedSymlinks[0] != "books/link" {
		t.Fatalf("SkippedSymlinks = %v, want [books/link]", result.SkippedSymlinks)
	}
	if len(result.SkippedSubmodules) != 1 || result.SkippedSubmodules[0] != "books/submod" {
		t.Fatalf("SkippedSubmodules = %v, want [books/submod]", result.SkippedSubmodules)
	}
	if result.APIRequests != 2 {
		t.Fatalf("APIRequests = %d, want 2", result.APIRequests)
	}
}

func TestWalkTreeExplicitRefSkipsDefaultBranchLookup(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{responses: map[string]string{
		"/repos/o/r/git/trees/v1?recursive=1": `{"truncated": false, "tree": [
			{"path": "a.txt", "mode": "100644", "type": "blob", "sha": "s", "size": 1}
		]}`,
	}}

	result, err := WalkTree(context.Background(), api, Address{Owner: "o", Repo: "r", Ref: "v1"})
	if err != nil {
		t.Fatalf("WalkTree unexpected error: %v", err)
	}
	if result.Ref != "v1" {
		t.Fatalf("Ref = %q, want v1", result.Ref)
	}
	if result.APIRequests != 1 {
		t.Fatalf("APIRequests = %d, want 1 (no default-branch lookup expected)", result.APIRequests)
	}
	if len(api.calls) != 1 || api.calls[0] != "/repos/o/r/git/trees/v1?recursive=1" {
		t.Fatalf("calls = %v, want exactly the tree call", api.calls)
	}
}

func TestWalkTreeTruncatedFallsBackToBFS(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{responses: map[string]string{
		"/repos/o/r/git/trees/main?recursive=1": `{"truncated": true, "tree": []}`,
		"/repos/o/r/git/trees/main":             `{"sha":"root","truncated":false,"tree":[{"path":"books","mode":"040000","type":"tree","sha":"booksSHA"}]}`,
		"/repos/o/r/git/trees/booksSHA": `{"sha":"booksSHA","truncated":false,"tree":[
			{"path":"foo.pdf","mode":"100644","type":"blob","sha":"s1","size":10},
			{"path":"sub","mode":"040000","type":"tree","sha":"subSHA"}
		]}`,
		"/repos/o/r/git/trees/subSHA": `{"sha":"subSHA","truncated":false,"tree":[
			{"path":"bar.pdf","mode":"100644","type":"blob","sha":"s2","size":20}
		]}`,
	}}

	result, err := WalkTree(context.Background(), api, Address{Owner: "o", Repo: "r", Ref: "main", Path: "books"})
	if err != nil {
		t.Fatalf("WalkTree unexpected error: %v", err)
	}
	// The bounded-BFS fallback exists precisely to produce a COMPLETE
	// listing despite the initial recursive=1 response being truncated;
	// Truncated on the final result only flips true if the BFS itself
	// hits the request cap or a nested subtree response is truncated —
	// neither happens in this fixture, so the walk is fully resolved.
	if result.Truncated {
		t.Fatal("Truncated = true, want false (BFS fallback completed without hitting the request cap)")
	}
	wantPaths := map[string]int64{"books/foo.pdf": 10, "books/sub/bar.pdf": 20}
	if len(result.Files) != len(wantPaths) {
		t.Fatalf("len(Files) = %d, want %d (got %+v)", len(result.Files), len(wantPaths), result.Files)
	}
	for _, f := range result.Files {
		wantSize, ok := wantPaths[f.Path]
		if !ok {
			t.Fatalf("unexpected file %q in result", f.Path)
		}
		if f.Size != wantSize {
			t.Fatalf("file %q size = %d, want %d", f.Path, f.Size, wantSize)
		}
	}
	// 1 (initial truncated recursive) + 1 (root, non-recursive) + 1 (books
	// subtree) + 1 (sub subtree) = 4.
	if result.APIRequests != 4 {
		t.Fatalf("APIRequests = %d, want 4", result.APIRequests)
	}
}

func TestWalkTreeSingleFileTarget(t *testing.T) {
	t.Parallel()

	api := &fakeAPI{responses: map[string]string{
		"/repos/o/r/git/trees/main?recursive=1": `{"truncated": false, "tree": [
			{"path": "README.md", "mode": "100644", "type": "blob", "sha": "s1", "size": 42}
		]}`,
	}}

	result, err := WalkTree(context.Background(), api, Address{Owner: "o", Repo: "r", Ref: "main", Path: "README.md"})
	if err != nil {
		t.Fatalf("WalkTree unexpected error: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "README.md" {
		t.Fatalf("Files = %+v, want a single README.md entry", result.Files)
	}
}

func TestWalkTreeBFSSingleFileSymlinkTargetIsSkipped(t *testing.T) {
	t.Parallel()

	// Truncated recursive response forces the BFS path; the target itself
	// is a symlink (mode 120000) and must land in SkippedSymlinks, not be
	// downloaded as a file.
	api := &fakeAPI{responses: map[string]string{
		"/repos/o/r/git/trees/main?recursive=1": `{"truncated": true, "tree": []}`,
		"/repos/o/r/git/trees/main": `{"truncated": false, "tree": [
			{"path": "link", "mode": "120000", "type": "blob", "sha": "s1", "size": 10}
		]}`,
	}}

	result, err := WalkTree(context.Background(), api, Address{Owner: "o", Repo: "r", Ref: "main", Path: "link"})
	if err != nil {
		t.Fatalf("WalkTree unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("Files = %+v, want none (symlink target must not be downloadable)", result.Files)
	}
	if len(result.SkippedSymlinks) != 1 || result.SkippedSymlinks[0] != "link" {
		t.Fatalf("SkippedSymlinks = %v, want [link]", result.SkippedSymlinks)
	}
}

// ---------------------------------------------------------------------
// HumanBytes
// ---------------------------------------------------------------------

func TestHumanBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		n    int64
		want string
	}{
		{"bytes", 500, "500 B"},
		{"exactly 1KB", 1024, "1.00 KB"},
		{"KB", 2048, "2.00 KB"},
		{"MB", 5 * 1024 * 1024, "5.00 MB"},
		{"GB", 2214592512, "2.06 GB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanBytes(tc.n)
			if got != tc.want {
				t.Fatalf("HumanBytes(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// SanitizeTerminal
// ---------------------------------------------------------------------

func TestSanitizeTerminal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"clean passthrough", "books/machine learning/foo.pdf", "books/machine learning/foo.pdf"},
		{"ansi escape stripped", "evil\x1b[2Jname.pdf", "evil[2Jname.pdf"},
		{"carriage return and newline stripped", "line1\r\nline2", "line1line2"},
		{"tab preserved", "a\tb", "a\tb"},
		{"bell stripped", "ding\x07dong", "dingdong"},
		{"DEL stripped", "a\x7fb", "ab"},
		{"C1 single-byte CSI stripped", "a2Jb", "a2Jb"},
		{"C1 range boundary stripped", "xyz", "xyz"},
		{"printable unicode above C1 preserved", "café — naïve", "café — naïve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeTerminal(tc.input)
			if got != tc.want {
				t.Fatalf("SanitizeTerminal(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// StreamToFile
// ---------------------------------------------------------------------

func TestStreamToFile(t *testing.T) {
	t.Parallel()

	t.Run("writes atomically and returns byte count", func(t *testing.T) {
		t.Parallel()
		dest := t.TempDir() + "/sub/out.txt"
		n, err := StreamToFile(strings.NewReader("hello\n"), dest, 6)
		if err != nil {
			t.Fatalf("StreamToFile unexpected error: %v", err)
		}
		if n != 6 {
			t.Fatalf("bytes = %d, want 6", n)
		}
		data, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("reading dest: %v", err)
		}
		if string(data) != "hello\n" {
			t.Fatalf("content = %q, want %q", data, "hello\n")
		}
	})

	t.Run("size mismatch removes temp and wraps ErrSizeMismatch", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dest := dir + "/out.txt"
		_, err := StreamToFile(strings.NewReader("hello\n"), dest, 999)
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if !errors.Is(err, ErrSizeMismatch) {
			t.Fatalf("error %v does not wrap ErrSizeMismatch", err)
		}
		if _, statErr := os.Stat(dest); statErr == nil {
			t.Fatal("dest exists after size-mismatch failure")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Fatalf("temp artifacts left behind: %v", entries)
		}
	})

	t.Run("skips size check when expectedSize negative", func(t *testing.T) {
		t.Parallel()
		dest := t.TempDir() + "/out.bin"
		n, err := StreamToFile(strings.NewReader("abc"), dest, -1)
		if err != nil {
			t.Fatalf("StreamToFile unexpected error: %v", err)
		}
		if n != 3 {
			t.Fatalf("bytes = %d, want 3", n)
		}
	})

	t.Run("no collision with a sibling file literally named dest.partial", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dest := dir + "/x"
		if err := os.WriteFile(dest+".partial", []byte("pre-existing"), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if _, err := StreamToFile(strings.NewReader("real"), dest, 4); err != nil {
			t.Fatalf("StreamToFile unexpected error: %v", err)
		}
		pre, err := os.ReadFile(dest + ".partial")
		if err != nil || string(pre) != "pre-existing" {
			t.Fatalf("sibling x.partial clobbered: content=%q err=%v", pre, err)
		}
	})
}

// ---------------------------------------------------------------------
// ComputeStats
// ---------------------------------------------------------------------

func TestComputeStats(t *testing.T) {
	t.Parallel()

	files := []TreeFile{
		{Path: "books/ml/deep.pdf", Size: 300},
		{Path: "books/ml/intro.pdf", Size: 100},
		{Path: "books/nlp/tokens.epub", Size: 250},
		{Path: "books/README.md", Size: 10},
		{Path: "books/notes", Size: 5},
	}

	stats := ComputeStats(files, "books", 2)

	if stats.TotalFiles != 5 {
		t.Fatalf("TotalFiles = %d, want 5", stats.TotalFiles)
	}
	if stats.TotalBytes != 665 {
		t.Fatalf("TotalBytes = %d, want 665", stats.TotalBytes)
	}

	wantFolders := []FolderStat{
		{Folder: "ml", Files: 2, Bytes: 400},
		{Folder: "nlp", Files: 1, Bytes: 250},
		{Folder: "(root)", Files: 2, Bytes: 15},
	}
	if len(stats.ByFolder) != len(wantFolders) {
		t.Fatalf("ByFolder = %+v, want %+v", stats.ByFolder, wantFolders)
	}
	for i, want := range wantFolders {
		if stats.ByFolder[i] != want {
			t.Fatalf("ByFolder[%d] = %+v, want %+v", i, stats.ByFolder[i], want)
		}
	}

	wantExts := []ExtStat{
		{Ext: ".pdf", Files: 2, Bytes: 400},
		{Ext: ".epub", Files: 1, Bytes: 250},
		{Ext: ".md", Files: 1, Bytes: 10},
		{Ext: "(none)", Files: 1, Bytes: 5},
	}
	if len(stats.ByExtension) != len(wantExts) {
		t.Fatalf("ByExtension = %+v, want %+v", stats.ByExtension, wantExts)
	}
	for i, want := range wantExts {
		if stats.ByExtension[i] != want {
			t.Fatalf("ByExtension[%d] = %+v, want %+v", i, stats.ByExtension[i], want)
		}
	}

	wantLargest := []FileStat{
		{Path: "ml/deep.pdf", Size: 300},
		{Path: "nlp/tokens.epub", Size: 250},
	}
	if len(stats.Largest) != 2 {
		t.Fatalf("Largest = %+v, want top 2", stats.Largest)
	}
	for i, want := range wantLargest {
		if stats.Largest[i] != want {
			t.Fatalf("Largest[%d] = %+v, want %+v", i, stats.Largest[i], want)
		}
	}
}

// ---------------------------------------------------------------------
// planJobs — unsafe-path diversion (per-file failure, not batch abort)
// ---------------------------------------------------------------------

func TestPlanJobsDivertsUnsafePaths(t *testing.T) {
	t.Parallel()

	d := &Downloader{}
	files := []TreeFile{
		{Path: "books/clean-one.pdf", SHA: "s1", Size: 10},
		{Path: "books/../../etc/passwd", SHA: "s2", Size: 20}, // traversal — rejected on every OS
		{Path: "books/clean-two.pdf", SHA: "s3", Size: 30},
	}

	jobs, failures := d.planJobs(Address{Owner: "o", Repo: "r", Path: "books"}, files, "/tmp/dest")

	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2 (clean files must still be planned; got %+v)", len(jobs), jobs)
	}
	for _, j := range jobs {
		if j.file.SHA == "s2" {
			t.Fatalf("unsafe file was planned: %+v", j)
		}
	}
	if len(failures) != 1 {
		t.Fatalf("len(failures) = %d, want 1 (got %+v)", len(failures), failures)
	}
	if failures[0].Path != "books/../../etc/passwd" {
		t.Fatalf("failure path = %q, want the unsafe remote path", failures[0].Path)
	}
	if failures[0].Error == "" {
		t.Fatal("failure carries no error message")
	}
}

// ---------------------------------------------------------------------
// checkBlobSHA — blob-API payload verification
// ---------------------------------------------------------------------

func TestCheckBlobSHA(t *testing.T) {
	t.Parallel()

	// "hello\n" has the known git blob SHA ce0136...
	const helloSHA = "ce013625030ba8dba906f756967f9e9ca394464a"

	if err := checkBlobSHA([]byte("hello\n"), helloSHA); err != nil {
		t.Fatalf("checkBlobSHA on matching content: unexpected error: %v", err)
	}
	err := checkBlobSHA([]byte("tampered\n"), helloSHA)
	if err == nil {
		t.Fatal("checkBlobSHA on mismatching content: want error, got nil")
	}
	if !strings.Contains(err.Error(), "SHA mismatch") {
		t.Fatalf("mismatch error %q does not name the SHA mismatch", err)
	}
}
