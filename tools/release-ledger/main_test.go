package main

import (
	"bytes"
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

func TestFirstParentCommitsExcludesSideBranchCommits(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "config", "user.email", "test@example.com")

	writeFile(t, repo, "README.md", "base\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	base := gitOutputIn(t, repo, "rev-parse", "HEAD")

	writeFile(t, repo, "main.txt", "main\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "main-line")
	mainLine := gitOutputIn(t, repo, "rev-parse", "HEAD")

	runGit(t, repo, "checkout", "-b", "side", base)
	writeFile(t, repo, "side.txt", "side\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "side-line")
	sideLine := gitOutputIn(t, repo, "rev-parse", "HEAD")

	runGit(t, repo, "checkout", "-B", "main", mainLine)
	runGit(t, repo, "merge", "--no-ff", sideLine, "-m", "merge-side")
	merge := gitOutputIn(t, repo, "rev-parse", "HEAD")
	writeFile(t, repo, "after.txt", "after\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "after-merge")
	afterMerge := gitOutputIn(t, repo, "rev-parse", "HEAD")

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

	got, err := firstParentCommits(base, afterMerge)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{mainLine, merge, afterMerge}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("firstParentCommits() = %v, want %v", got, want)
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
		{"nested module version.go", "library/ai/v0/vzero/internal/cli/version.go", true},
		{"nested module mcp main.go", "library/ai/v0/vzero/cmd/v0-pp-mcp/main.go", true},
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

func TestResolveModuleDirMatchesShell(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		wantRel string
		wantErr bool
	}{
		{
			name: "catalog go.mod ignores goreleaser dir",
			files: map[string]string{
				"go.mod":               "module example.com/cli\n",
				".goreleaser.yaml":     "builds:\n  - dir: ./nested\n",
				"nested/go.mod":        "module example.com/cli/nested\n",
				"internal/cli/root.go": "package cli\n\nvar version = \"1\"\n",
			},
			wantRel: ".",
		},
		{
			name: "goreleaser dir when catalog has no go.mod",
			files: map[string]string{
				".goreleaser.yaml": "version: 2\nbuilds:\n  - id: cli\n    dir: ./vzero\n  - id: mcp\n    dir: \"./vzero\"\n",
				"vzero/go.mod":     "module example.com/vzero\n",
			},
			wantRel: "vzero",
		},
		{
			name: "inline list item dir",
			files: map[string]string{
				".goreleaser.yaml": "builds:\n  - dir: ./vzero\n",
				"vzero/README.md":  "module lives here\n",
			},
			wantRel: "vzero",
		},
		{
			name: "missing goreleaser dir falls back to the nested module",
			files: map[string]string{
				".goreleaser.yaml": "builds:\n  - dir: ./missing\n",
				"mod/go.mod":       "module example.com/mod\n",
			},
			wantRel: "mod",
		},
		{
			name: "unsafe goreleaser dir fails",
			files: map[string]string{
				".goreleaser.yaml": "builds:\n  - dir: ../escape\n",
			},
			wantErr: true,
		},
		{
			name: "single nested go.mod when goreleaser has no dir",
			files: map[string]string{
				".goreleaser.yaml": "version: 2\nbuilds:\n  - id: cli\n",
				"mod/go.mod":       "module example.com/mod\n",
				"other/README.md":  "not a module\n",
			},
			wantRel: "mod",
		},
		{
			name: "ambiguous nested modules stay at the catalog root",
			files: map[string]string{
				"one/go.mod": "module example.com/one\n",
				"two/go.mod": "module example.com/two\n",
			},
			wantRel: ".",
		},
		{
			name: "disagreeing goreleaser dirs fail",
			files: map[string]string{
				".goreleaser.yaml": "builds:\n  - dir: ./one\n  - dir: ./two\n",
				"one/go.mod":       "module example.com/one\n",
				"two/go.mod":       "module example.com/two\n",
			},
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			catalog := filepath.Join(t.TempDir(), "cli")
			for rel, body := range tt.files {
				writeFile(t, catalog, rel, body)
			}
			got, err := resolveModuleDir(catalog)
			script, scriptErr := resolveModuleDirShell(t, catalog)
			if tt.wantErr {
				if err == nil || scriptErr == nil {
					t.Fatalf("expected both resolvers to fail, go=%v shell=%v", err, scriptErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if scriptErr != nil {
				t.Fatal(scriptErr)
			}
			want := catalog
			if tt.wantRel != "." {
				want = filepath.Join(catalog, tt.wantRel)
			}
			if got != want {
				t.Fatalf("resolveModuleDir() = %q, want %q", got, want)
			}
			if script != want {
				t.Fatalf("resolve-module-dir.sh = %q, want %q", script, want)
			}
		})
	}
}

func TestStampRuntimeVersionUsesNestedModuleAndIgnoresDecoy(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "library/ai/v0/.goreleaser.yaml", "version: 2\nbuilds:\n  - id: cli\n    dir: ./vzero\n  - id: mcp\n    dir: ./vzero\n")
	writeFile(t, repo, "library/ai/v0/vzero/go.mod", "module example.com/vzero\n")
	writeFile(t, repo, "library/ai/v0/vzero/internal/cli/version.go", "package cli\n\nvar version = \"2026.8.1\"\n")
	writeFile(t, repo, "library/ai/v0/vzero/cmd/v0-pp-mcp/main.go", "package main\n\nvar version = \"2026.8.1\"\n")
	writeFile(t, repo, "library/ai/v0/internal/cli/version.go", "package cli\n\nvar version = \"decoy\"\n")

	dir := filepath.Join(repo, "library", "ai", "v0")
	changed, err := stampRuntimeVersion(dir, releaseManifest{Version: "2026.9.1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected nested version files to change")
	}
	if got := readFile(t, repo, "library/ai/v0/vzero/internal/cli/version.go"); !strings.Contains(got, `var version = "2026.9.1"`) {
		t.Fatalf("nested version.go was not stamped:\n%s", got)
	}
	if got := readFile(t, repo, "library/ai/v0/vzero/cmd/v0-pp-mcp/main.go"); !strings.Contains(got, `var version = "2026.9.1"`) {
		t.Fatalf("nested mcp main.go was not stamped:\n%s", got)
	}
	if got := readFile(t, repo, "library/ai/v0/internal/cli/version.go"); strings.Contains(got, "2026.9.1") {
		t.Fatalf("catalog-root decoy was stamped:\n%s", got)
	}
}

func TestStampRuntimeVersionNormalCLIIgnoresNestedDir(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "library/health/grants/go.mod", "module example.com/grants\n")
	writeFile(t, repo, "library/health/grants/.goreleaser.yaml", "builds:\n  - dir: ./nested\n")
	writeFile(t, repo, "library/health/grants/internal/cli/root.go", "package cli\n\nvar version = \"2026.10.1\"\n")
	writeFile(t, repo, "library/health/grants/nested/go.mod", "module example.com/grants/nested\n")
	writeFile(t, repo, "library/health/grants/nested/internal/cli/version.go", "package cli\n\nvar version = \"leave-me\"\n")

	dir := filepath.Join(repo, "library", "health", "grants")
	changed, err := stampRuntimeVersion(dir, releaseManifest{Version: "2026.10.2"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected catalog-root version to change")
	}
	if got := readFile(t, repo, "library/health/grants/internal/cli/root.go"); !strings.Contains(got, `var version = "2026.10.2"`) {
		t.Fatalf("root.go was not stamped:\n%s", got)
	}
	if got := readFile(t, repo, "library/health/grants/nested/internal/cli/version.go"); strings.Contains(got, "2026.10.2") {
		t.Fatalf("nested module was stamped on a normal CLI:\n%s", got)
	}
}

func TestResolveAndStampRealCatalogsCheckMode(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	v0 := filepath.Join(repoRoot, "library", "ai", "v0")
	got, err := resolveModuleDir(v0)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(v0, "vzero")
	if got != want {
		t.Fatalf("ai/v0 module dir = %q, want %q", got, want)
	}
	shell, err := resolveModuleDirShell(t, v0)
	if err != nil {
		t.Fatal(err)
	}
	if shell != want {
		t.Fatalf("ai/v0 shell module dir = %q, want %q", shell, want)
	}
	assertCheckModeFindsVersion(t, v0,
		"library/ai/v0/vzero/internal/cli/version.go",
		"library/ai/v0/vzero/cmd/v0-pp-mcp/main.go",
	)

	for _, rel := range []string{"library/health/grants", "library/payments/kalshi"} {
		catalog := filepath.Join(repoRoot, rel)
		got, err = resolveModuleDir(catalog)
		if err != nil {
			t.Fatal(err)
		}
		if got != catalog {
			t.Fatalf("%s module dir = %q, want the catalog root", rel, got)
		}
		shell, err = resolveModuleDirShell(t, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if shell != catalog {
			t.Fatalf("%s shell module dir = %q, want the catalog root", rel, shell)
		}
	}
	assertCheckModeFindsVersion(t, filepath.Join(repoRoot, "library", "health", "grants"), "library/health/grants/internal/cli/root.go")
	assertCheckModeFindsVersion(t, filepath.Join(repoRoot, "library", "payments", "kalshi"), "library/payments/kalshi/internal/cli/root.go")
}

func assertCheckModeFindsVersion(t *testing.T, catalog string, rels ...string) {
	t.Helper()
	before := make([][]byte, len(rels))
	for i, rel := range rels {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		before[i] = data
	}
	changed, err := stampRuntimeVersion(catalog, releaseManifest{
		Version:      "2099.1.1",
		SourceCommit: "abc123",
		ReleasedAt:   "2099-01-01T00:00:00Z",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("check mode did not see a version literal under %s", catalog)
	}
	for i, rel := range rels {
		after, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before[i], after) {
			t.Fatalf("check mode wrote %s", rel)
		}
	}
}

func TestGoRunMainGoCompiles(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run main.go --help: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("-list-first-parent")) {
		t.Fatalf("go run main.go --help missing flags:\n%s", out)
	}
}

func resolveModuleDirShell(t *testing.T, catalog string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "resolve-module-dir.sh", catalog)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
