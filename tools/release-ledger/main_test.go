package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNextVersion(t *testing.T) {
	releasedAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		current string
		want    string
	}{
		{name: "initial", want: "2026.6.1"},
		{name: "same month increments", current: "2026.6.3", want: "2026.6.4"},
		{name: "new month resets", current: "2026.5.9", want: "2026.6.1"},
		{name: "new year resets", current: "2025.12.9", want: "2026.6.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextVersion(tt.current, releasedAt)
			if err != nil {
				t.Fatalf("nextVersion() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("nextVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextReleaseVersionIsIdempotentForSameSourceCommit(t *testing.T) {
	releasedAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	got, err := nextReleaseVersion(releaseManifest{
		Version:      "2026.6.3",
		SourceCommit: "abc123",
	}, options{releasedAt: releasedAt, sourceCommit: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026.6.3" {
		t.Fatalf("nextReleaseVersion() = %q, want existing version", got)
	}

	got, err = nextReleaseVersion(releaseManifest{
		Version:      "2026.6.3",
		SourceCommit: "abc123",
	}, options{releasedAt: releasedAt, sourceCommit: "def456"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026.6.4" {
		t.Fatalf("nextReleaseVersion() = %q, want incremented version", got)
	}
}

func TestIsRuntimeVersionPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"cli root.go", "library/social/x/internal/cli/root.go", true},
		{"cli version.go", "library/social/x/internal/cli/version.go", true},
		{"mcp main.go", "library/social/x/cmd/x-pp-mcp/main.go", true},
		{"mcp main.go nested slug", "library/category/my-tool/cmd/my-tool-pp-mcp/main.go", true},
		{"cli main.go", "library/social/x/cmd/x-pp-cli/main.go", false},
		{"non-mcp main.go suffix", "library/social/x/cmd/x-pp-mcpd/main.go", false},
		{"non-library mcp main.go", "tools/social/x/cmd/x-pp-mcp/main.go", false},
		{"mcp main.go in subdir", "library/social/x/cmd/x-pp-mcp/subdir/main.go", false},
		{"other go file", "library/social/x/internal/cli/other.go", false},
		{"random file", "library/social/x/README.md", false},
		{"release manifest", "library/social/x/.printing-press-release.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRuntimeVersionPath(tt.path)
			if got != tt.want {
				t.Errorf("isRuntimeVersionPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestChangedCLISlugsIgnoresReleaseLedgerFiles(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "config", "user.email", "test@example.com")

	writeFile(t, repo, "library/social/x/.printing-press.json", "{}\n")
	writeFile(t, repo, "library/social/x/README.md", "old\n")
	writeFile(t, repo, "library/social/y/.printing-press.json", "{}\n")
	writeFile(t, repo, "library/social/y/README.md", "old\n")
	writeFile(t, repo, "library/social/z/.printing-press.json", "{}\n")
	writeFile(t, repo, "library/social/z/internal/cli/root.go", "package cli\n\nvar version = \"1.0.0\"\n")
	writeFile(t, repo, "library/social/real/.printing-press.json", "{}\n")
	writeFile(t, repo, "library/social/real/internal/cli/root.go", "package cli\n\nvar version = \"1.0.0\"\n\nfunc run() {}\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	base := gitOutputIn(t, repo, "rev-parse", "HEAD")

	writeFile(t, repo, "library/social/x/CHANGELOG.md", "# Changelog\n")
	writeFile(t, repo, "library/social/x/.printing-press-release.json", "{}\n")
	writeFile(t, repo, "library/social/y/README.md", "new\n")
	writeFile(t, repo, "library/social/z/internal/cli/root.go", "package cli\n\nvar version = \"2026.6.1\"\n")
	writeFile(t, repo, "library/social/real/internal/cli/root.go", "package cli\n\nvar version = \"2026.6.1\"\n\nfunc run() { println(\"changed\") }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "update")
	head := gitOutputIn(t, repo, "rev-parse", "HEAD")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})

	got, err := changedCLIKeys(base, head)
	if err != nil {
		t.Fatal(err)
	}
	if got["social/x"] {
		t.Fatalf("release-ledger-only changes should be ignored: %#v", got)
	}
	if !got["social/y"] {
		t.Fatalf("non-release library change should select slug y: %#v", got)
	}
	if got["social/z"] {
		t.Fatalf("runtime-version-only changes should be ignored: %#v", got)
	}
	if !got["social/real"] {
		t.Fatalf("root.go with non-version changes should select slug real: %#v", got)
	}
}

func TestInsertChangelogEntryPreservesPrologue(t *testing.T) {
	existing := []byte(`# Changelog

All notable changes are documented here.

## v0.1.0 - 2026-05-01

- Existing entry.
`)
	entry := []byte("## 2026.6.1 - 2026-06-08\n\n- Baseline release metadata added for this published CLI.\n\n")

	got := string(insertChangelogEntry(existing, entry))
	want := `# Changelog

All notable changes are documented here.

## 2026.6.1 - 2026-06-08

- Baseline release metadata added for this published CLI.

## v0.1.0 - 2026-05-01

- Existing entry.
`
	if got != want {
		t.Fatalf("insertChangelogEntry mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestUpdateChangelogSkipsExistingReleaseSection(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "library/social/x/CHANGELOG.md", "# Changelog\n\n## 2026.6.1 - 2026-06-08\n\n- Existing.\n")
	changed, err := updateChangelog(
		filepath.Join(repo, "library", "social", "x"),
		releaseManifest{Version: "2026.6.1"},
		options{releasedAt: time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC), changeTitle: "Existing"},
		true,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("updateChangelog should not duplicate an existing release section")
	}
}

func TestUpdateCLIInitializesManifestChangelogAndRuntimeVersion(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "library", "social", "x-twitter")
	writeFile(t, repo, "library/social/x-twitter/.printing-press.json", `{
  "api_name": "x-twitter",
  "cli_name": "x-twitter-pp-cli",
  "printing_press_version": "4.20.1",
  "run_id": "20260603-230951"
}
`)
	writeFile(t, repo, "library/social/x-twitter/internal/cli/root.go", `package cli

var version = "1.0.0"
var other = "leave-me"
`)

	releasedAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	result, err := updateCLI(dir, options{
		initMissing:  true,
		releasedAt:   releasedAt,
		sourceCommit: "abc123",
		changeTitle:  "Baseline release metadata added for this published CLI.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.changed || result.version != "2026.6.1" {
		t.Fatalf("unexpected result: %#v", result)
	}

	manifest := readFile(t, repo, "library/social/x-twitter/.printing-press-release.json")
	for _, want := range []string{
		`"version": "2026.6.1"`,
		`"source_commit": "abc123"`,
		`"printing_press_version": "4.20.1"`,
	} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("release manifest missing %q:\n%s", want, manifest)
		}
	}
	changelog := readFile(t, repo, "library/social/x-twitter/CHANGELOG.md")
	if !strings.Contains(changelog, "## 2026.6.1 - 2026-06-08") {
		t.Fatalf("changelog missing release entry:\n%s", changelog)
	}
	root := readFile(t, repo, "library/social/x-twitter/internal/cli/root.go")
	if !strings.Contains(root, `var version = "2026.6.1"`) {
		t.Fatalf("root.go was not stamped:\n%s", root)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func gitOutputIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
