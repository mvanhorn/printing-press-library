// release-ledger maintains per-CLI release metadata for the published library.
//
// The library deliberately assigns these versions after a PR merges instead of
// asking contributors to edit 233 independent changelogs by hand. Versions use
// YYYY.M.N, where N is the release count for that CLI within the month.
//
// Usage, from the repository root:
//
//	go run ./tools/release-ledger/main.go --init-missing
//	go run ./tools/release-ledger/main.go --changed-from <sha> --changed-to <sha>
//	go run ./tools/release-ledger/main.go --check --changed-from <sha> --changed-to <sha>
//	go run ./tools/release-ledger/main.go --list-first-parent --changed-from <sha> --changed-to <sha>
//	go run ./tools/release-ledger/main.go --restamp [--check] [<category>/<slug> ...]
//
// resolveModuleDir is in this file on purpose. `go run` of a single file does
// not compile its siblings, and the workflow and docs pin that invocation.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	libraryDir          = "library"
	releaseManifestName = ".printing-press-release.json"
	changelogName       = "CHANGELOG.md"
)

var versionRe = regexp.MustCompile(`^(\d{4})\.(\d{1,2})\.(\d+)$`)
var varStringRe = regexp.MustCompile(`(?m)(\bvar\s+%s\s*=\s*)"[^"]*"`)

type releaseManifest struct {
	SchemaVersion        int             `json:"schema_version"`
	Slug                 string          `json:"slug"`
	CLIName              string          `json:"cli_name,omitempty"`
	Version              string          `json:"version"`
	ReleasedAt           string          `json:"released_at"`
	SourceCommit         string          `json:"source_commit"`
	PrintingPressVersion string          `json:"printing_press_version,omitempty"`
	RunID                string          `json:"run_id,omitempty"`
	Changes              []releaseChange `json:"changes"`
}

type releaseChange struct {
	Title  string `json:"title"`
	PR     int    `json:"pr,omitempty"`
	URL    string `json:"url,omitempty"`
	Commit string `json:"commit,omitempty"`
}

type printingPressManifest struct {
	APIName              string `json:"api_name"`
	CLIName              string `json:"cli_name"`
	PrintingPressVersion string `json:"printing_press_version"`
	RunID                string `json:"run_id"`
}

type options struct {
	initMissing     bool
	restamp         bool
	restampKeys     []string
	check           bool
	listFirstParent bool
	changedFrom     string
	changedTo       string
	releasedAt      time.Time
	sourceCommit    string
	changeTitle     string
	changePR        int
	changeURL       string
}

type updateResult struct {
	dir     string
	version string
	changed bool
}

func main() {
	opts, err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}
	if opts.listFirstParent {
		commits, err := firstParentCommits(opts.changedFrom, opts.changedTo)
		if err != nil {
			log.Fatal(err)
		}
		for _, commit := range commits {
			fmt.Println(commit)
		}
		return
	}

	var results []updateResult
	if opts.restamp {
		results, err = restampCLIs(libraryDir, opts.restampKeys, opts.check)
	} else {
		results, err = run(opts)
	}
	if err != nil {
		log.Fatal(err)
	}

	var changed []updateResult
	for _, result := range results {
		if result.changed {
			changed = append(changed, result)
		}
	}
	if opts.check {
		if len(changed) > 0 {
			var paths []string
			for _, result := range changed {
				paths = append(paths, fmt.Sprintf("%s -> %s", result.dir, result.version))
			}
			if opts.restamp {
				log.Fatalf("runtime version drift detected:\n%s\nRun `go run ./tools/release-ledger/main.go --restamp` with the same CLIs and commit the result.", strings.Join(paths, "\n"))
			}
			log.Fatalf("release ledger drift detected:\n%s\nRun `go run ./tools/release-ledger/main.go` with the same scope and commit the result.", strings.Join(paths, "\n"))
		}
		if opts.restamp {
			fmt.Fprintln(os.Stderr, "runtime versions match the release ledger")
			return
		}
		fmt.Fprintln(os.Stderr, "release ledger is in sync")
		return
	}
	if opts.restamp {
		fmt.Fprintf(os.Stderr, "restamped %d CLIs\n", len(changed))
		return
	}
	fmt.Fprintf(os.Stderr, "updated %d CLI release ledger entries\n", len(changed))
}

func parseFlags() (options, error) {
	var opts options
	var releasedAt string
	flag.BoolVar(&opts.initMissing, "init-missing", false, "initialize CLIs missing release metadata")
	flag.BoolVar(&opts.restamp, "restamp", false, "re-stamp runtime version vars from each CLI's existing release ledger without cutting a release; optional arguments name <category>/<slug> CLIs")
	flag.BoolVar(&opts.check, "check", false, "exit non-zero if the selected release ledger entries would change")
	flag.BoolVar(&opts.listFirstParent, "list-first-parent", false, "list first-parent commits in the selected range")
	flag.StringVar(&opts.changedFrom, "changed-from", "", "git ref to diff from when selecting changed CLIs")
	flag.StringVar(&opts.changedTo, "changed-to", "", "git ref to diff to when selecting changed CLIs")
	flag.StringVar(&releasedAt, "released-at", "", "release timestamp (RFC3339); defaults to now")
	flag.StringVar(&opts.sourceCommit, "source-commit", "", "source commit SHA; defaults to HEAD")
	flag.StringVar(&opts.changeTitle, "change-title", "", "human summary for the changelog entry")
	flag.IntVar(&opts.changePR, "change-pr", 0, "pull request number for the changelog entry")
	flag.StringVar(&opts.changeURL, "change-url", "", "pull request URL for the changelog entry")
	flag.Parse()

	if opts.restamp {
		if opts.initMissing || opts.listFirstParent || opts.changedFrom != "" || opts.changedTo != "" {
			return opts, errors.New("--restamp cannot be combined with --init-missing/--list-first-parent/--changed-from/--changed-to")
		}
		opts.restampKeys = flag.Args()
		return opts, nil
	}
	if flag.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments %q; only --restamp takes CLI names", flag.Args())
	}
	if opts.initMissing && (opts.changedFrom != "" || opts.changedTo != "") {
		return opts, errors.New("--init-missing cannot be combined with --changed-from/--changed-to")
	}
	if opts.listFirstParent && (opts.initMissing || opts.check) {
		return opts, errors.New("--list-first-parent cannot be combined with --init-missing/--check")
	}
	if !opts.initMissing && (opts.changedFrom == "" || opts.changedTo == "") {
		return opts, errors.New("use --init-missing or provide both --changed-from and --changed-to")
	}
	if releasedAt == "" {
		opts.releasedAt = time.Now().UTC()
	} else {
		parsed, err := time.Parse(time.RFC3339, releasedAt)
		if err != nil {
			return opts, fmt.Errorf("parsing --released-at: %w", err)
		}
		opts.releasedAt = parsed.UTC()
	}
	if opts.sourceCommit == "" {
		commit, err := gitOutput("rev-parse", "HEAD")
		if err != nil {
			return opts, fmt.Errorf("resolving HEAD: %w", err)
		}
		opts.sourceCommit = commit
	}
	if opts.changeTitle == "" {
		if opts.initMissing {
			opts.changeTitle = "Baseline release metadata added for this published CLI."
		} else {
			opts.changeTitle = "Published library update."
		}
	}
	return opts, nil
}

func run(opts options) ([]updateResult, error) {
	dirs, err := selectCLIDirs(opts)
	if err != nil {
		return nil, err
	}
	var results []updateResult
	for _, dir := range dirs {
		result, err := updateCLI(dir, opts)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func selectCLIDirs(opts options) ([]string, error) {
	all, err := listCLIDirs(libraryDir)
	if err != nil {
		return nil, err
	}
	if opts.initMissing {
		var out []string
		for _, dir := range all {
			if !exists(filepath.Join(dir, releaseManifestName)) || !exists(filepath.Join(dir, changelogName)) {
				out = append(out, dir)
			}
		}
		return out, nil
	}

	changed, err := changedCLIKeys(opts.changedFrom, opts.changedTo)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, dir := range all {
		if changed[cliKeyFromDir(dir)] {
			out = append(out, dir)
		}
	}
	return out, nil
}

// restampCLIs rewrites runtime version vars to match each CLI's existing
// release ledger, so a stamping fix reaches CLIs that have not changed since
// their last release. It never writes the ledger or changelog. keys are
// "<category>/<slug>", optionally prefixed with "library/"; none means all.
func restampCLIs(root string, keys []string, check bool) ([]updateResult, error) {
	dirs, err := listCLIDirs(root)
	if err != nil {
		return nil, err
	}
	if len(keys) > 0 {
		byKey := make(map[string]string, len(dirs))
		for _, dir := range dirs {
			byKey[cliKeyFromDir(dir)] = dir
		}
		var selected []string
		for _, key := range keys {
			if strings.HasPrefix(key, "-") {
				return nil, fmt.Errorf("--restamp: put flags before CLI names, got %q", key)
			}
			normalized := strings.Trim(strings.TrimPrefix(filepath.ToSlash(key), libraryDir+"/"), "/")
			dir, ok := byKey[normalized]
			if !ok {
				return nil, fmt.Errorf("--restamp: no published CLI %q", key)
			}
			selected = append(selected, dir)
		}
		dirs = uniqueStrings(selected)
	}
	var results []updateResult
	for _, dir := range dirs {
		manifest, err := readReleaseManifest(filepath.Join(dir, releaseManifestName))
		if err != nil {
			return results, err
		}
		if manifest.Version == "" {
			continue
		}
		changed, err := stampRuntimeVersion(dir, manifest, check)
		if err != nil {
			return results, fmt.Errorf("%s: %w", dir, err)
		}
		results = append(results, updateResult{dir: dir, version: manifest.Version, changed: changed})
	}
	return results, nil
}

func listCLIDirs(root string) ([]string, error) {
	categories, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, category := range categories {
		if !category.IsDir() {
			continue
		}
		categoryPath := filepath.Join(root, category.Name())
		entries, err := os.ReadDir(categoryPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(categoryPath, entry.Name())
			if exists(filepath.Join(dir, ".printing-press.json")) {
				dirs = append(dirs, dir)
			}
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func firstParentCommits(from, to string) ([]string, error) {
	out, err := gitOutput("rev-list", "--first-parent", "--reverse", from+".."+to)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func changedCLIKeys(from, to string) (map[string]bool, error) {
	out := make(map[string]bool)
	diff, err := gitOutput("diff", "--name-only", "--diff-filter=ACMRT", from, to, "--", libraryDir)
	if err != nil {
		return out, err
	}
	// A deleted file never selects a CLI on its own (see --diff-filter above),
	// but it does cancel the homepage-only exemption for that CLI, so a PR
	// that edits homepage and deletes source still cuts a release.
	// --no-renames reports a file moved out to another CLI as a deletion here.
	deleted, err := gitOutput("diff", "--name-only", "--no-renames", "--diff-filter=D", from, to, "--", libraryDir)
	if err != nil {
		return out, err
	}
	hasDeletion := make(map[string]bool)
	for _, name := range strings.Split(deleted, "\n") {
		if key := cliKeyFromPath(strings.TrimSpace(name)); key != "" && !isReleaseLedgerGeneratedPath(name) {
			hasDeletion[key] = true
		}
	}
	for _, name := range strings.Split(diff, "\n") {
		name = strings.TrimSpace(name)
		if name == "" || isReleaseLedgerGeneratedPath(name) {
			continue
		}
		if isRuntimeVersionPath(name) {
			onlyStamp, err := isRuntimeVersionOnlyChange(from, to, name)
			if err != nil {
				return out, err
			}
			if onlyStamp {
				continue
			}
		}
		if strings.HasSuffix(name, "/.printing-press.json") && !hasDeletion[cliKeyFromPath(name)] {
			onlyHomepage, err := isHomepageOnlyChange(from, to, name)
			if err != nil {
				return out, err
			}
			if onlyHomepage {
				continue
			}
		}
		if key := cliKeyFromPath(name); key != "" {
			out[key] = true
		}
	}
	return out, nil
}

// cliKeyFromPath returns "<category>/<slug>" for a path under library/, or ""
// for any other path.
func cliKeyFromPath(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) >= 3 && parts[0] == libraryDir {
		return parts[1] + "/" + parts[2]
	}
	return ""
}

func isReleaseLedgerGeneratedPath(name string) bool {
	return strings.HasSuffix(name, "/"+releaseManifestName) ||
		strings.HasSuffix(name, "/"+changelogName)
}

func isRuntimeVersionPath(name string) bool {
	if strings.HasSuffix(name, "/internal/cli/root.go") || strings.HasSuffix(name, "/internal/cli/version.go") {
		return true
	}
	parts := strings.Split(name, "/")
	return len(parts) >= 5 && parts[0] == libraryDir && parts[len(parts)-1] == "main.go" &&
		parts[len(parts)-3] == "cmd" && strings.HasSuffix(parts[len(parts)-2], "-pp-mcp")
}

func isRuntimeVersionOnlyChange(from, to, path string) (bool, error) {
	diff, err := gitOutput("diff", "--unified=0", from, to, "--", path)
	if err != nil {
		return false, err
	}
	changed := false
	for _, line := range strings.Split(diff, "\n") {
		if line == "" || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		changed = true
		text := strings.TrimSpace(line[1:])
		if !isRuntimeVersionStampLine(text) {
			return false, nil
		}
	}
	return changed, nil
}

// isHomepageOnlyChange reports whether a .printing-press.json edit touches
// nothing but the catalog-only `homepage` field. Such edits change registry
// metadata, not the CLI, so they must not cut a release. Anything doubtful
// counts as a real change: a manifest that did not exist at `from` (a new
// CLI), or one that is not valid JSON on either side.
func isHomepageOnlyChange(from, to, path string) (bool, error) {
	if _, err := gitOutput("cat-file", "-e", from+":"+path); err != nil {
		return false, nil
	}
	before, ok, err := manifestWithoutHomepage(from, path)
	if err != nil || !ok {
		return false, err
	}
	after, ok, err := manifestWithoutHomepage(to, path)
	if err != nil || !ok {
		return false, err
	}
	return bytes.Equal(before, after), nil
}

// manifestWithoutHomepage returns the manifest at rev with `homepage` removed,
// re-encoded with sorted keys so formatting differences compare equal. ok is
// false when the manifest is not a JSON object.
func manifestWithoutHomepage(rev, path string) (out []byte, ok bool, err error) {
	// Read untrimmed: gitOutput's TrimSpace would strip invalid bytes such as
	// a non-breaking space after the closing brace before the EOF check.
	cmd := exec.Command("git", "show", rev+":"+path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, false, fmt.Errorf("git show %s:%s: %w: %s", rev, path, err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var fields map[string]any
	if err := dec.Decode(&fields); err != nil || fields == nil {
		return nil, false, nil
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false, nil // trailing data after the object
	}
	delete(fields, "homepage")
	out, err = json.Marshal(fields)
	return out, err == nil, err
}

func isRuntimeVersionStampLine(line string) bool {
	for _, name := range []string{"version", "commit", "date"} {
		re := regexp.MustCompile(fmt.Sprintf(`^var\s+%s\s*=\s*"[^"]*"$`, regexp.QuoteMeta(name)))
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func updateCLI(dir string, opts options) (updateResult, error) {
	pp, err := readPrintingPressManifest(filepath.Join(dir, ".printing-press.json"))
	if err != nil {
		return updateResult{dir: dir}, err
	}
	current, err := readReleaseManifest(filepath.Join(dir, releaseManifestName))
	if err != nil {
		return updateResult{dir: dir}, err
	}
	nextVersion, err := nextReleaseVersion(current, opts)
	if err != nil {
		return updateResult{dir: dir}, fmt.Errorf("%s: %w", dir, err)
	}
	next := releaseManifest{
		SchemaVersion:        1,
		Slug:                 slugFromDir(dir),
		CLIName:              pp.CLIName,
		Version:              nextVersion,
		ReleasedAt:           opts.releasedAt.Format(time.RFC3339),
		SourceCommit:         opts.sourceCommit,
		PrintingPressVersion: pp.PrintingPressVersion,
		RunID:                pp.RunID,
		Changes: []releaseChange{{
			Title:  opts.changeTitle,
			PR:     opts.changePR,
			URL:    opts.changeURL,
			Commit: opts.sourceCommit,
		}},
	}

	changed := false
	if changedFile, err := writeJSONIfChanged(filepath.Join(dir, releaseManifestName), next, opts.check); err != nil {
		return updateResult{dir: dir}, err
	} else if changedFile {
		changed = true
	}
	if changedFile, err := updateChangelog(dir, next, opts, current.Version != "", opts.check); err != nil {
		return updateResult{dir: dir}, err
	} else if changedFile {
		changed = true
	}
	if changedFile, err := stampRuntimeVersion(dir, next, opts.check); err != nil {
		return updateResult{dir: dir}, err
	} else if changedFile {
		changed = true
	}
	return updateResult{dir: dir, version: nextVersion, changed: changed}, nil
}

func readPrintingPressManifest(path string) (printingPressManifest, error) {
	var manifest printingPressManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parsing %s: %w", path, err)
	}
	return manifest, nil
}

func readReleaseManifest(path string) (releaseManifest, error) {
	var manifest releaseManifest
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return manifest, nil
	}
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parsing %s: %w", path, err)
	}
	return manifest, nil
}

func nextReleaseVersion(current releaseManifest, opts options) (string, error) {
	if current.Version != "" && current.SourceCommit == opts.sourceCommit {
		return current.Version, nil
	}
	return nextVersion(current.Version, opts.releasedAt)
}

func nextVersion(current string, releasedAt time.Time) (string, error) {
	year, month, sequence := releasedAt.Date()
	if current == "" {
		return fmt.Sprintf("%d.%d.1", year, int(month)), nil
	}
	matches := versionRe.FindStringSubmatch(current)
	if matches == nil {
		return "", fmt.Errorf("invalid release version %q, want YYYY.M.N", current)
	}
	currentYear, _ := strconv.Atoi(matches[1])
	currentMonth, _ := strconv.Atoi(matches[2])
	currentSequence, _ := strconv.Atoi(matches[3])
	if currentYear == year && currentMonth == int(month) {
		sequence = currentSequence + 1
	} else {
		sequence = 1
	}
	return fmt.Sprintf("%d.%d.%d", year, int(month), sequence), nil
}

func writeJSONIfChanged(path string, value any, check bool) (bool, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return false, err
	}
	data = append(data, '\n')
	return writeIfChanged(path, data, check)
}

func updateChangelog(dir string, manifest releaseManifest, opts options, hasPriorRelease bool, check bool) (bool, error) {
	path := filepath.Join(dir, changelogName)
	entry := changelogEntry(manifest, opts, hasPriorRelease)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte("# Changelog\n\nThis file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.\n\n")
	} else if err != nil {
		return false, err
	} else if bytes.Contains(data, []byte("## "+manifest.Version+" - ")) {
		return false, nil
	}
	next := insertChangelogEntry(data, entry)
	return writeIfChanged(path, next, check)
}

func changelogEntry(manifest releaseManifest, opts options, hasPriorRelease bool) []byte {
	var line string
	if hasPriorRelease {
		title := strings.TrimSpace(opts.changeTitle)
		line = fmt.Sprintf("- %s", title)
		if opts.changePR > 0 {
			line += fmt.Sprintf(" (#%d)", opts.changePR)
		}
		if !strings.HasSuffix(line, ".") && !strings.HasSuffix(line, "!") && !strings.HasSuffix(line, "?") {
			line += "."
		}
	} else {
		line = "- Baseline release metadata added for this published CLI."
	}
	return []byte(fmt.Sprintf("## %s - %s\n\n%s\n\n", manifest.Version, opts.releasedAt.Format("2006-01-02"), line))
}

func insertChangelogEntry(existing, entry []byte) []byte {
	normalized := bytes.TrimRight(existing, " \t\r\n")
	if len(normalized) == 0 {
		normalized = []byte("# Changelog")
	}
	normalized = append(normalized, '\n')
	firstSection := bytes.Index(normalized, []byte("\n## "))
	if firstSection < 0 {
		out := append([]byte{}, normalized...)
		out = append(out, '\n')
		out = append(out, entry...)
		return out
	}
	insertAt := firstSection + 1
	out := append([]byte{}, normalized[:insertAt]...)
	out = append(out, entry...)
	out = append(out, normalized[insertAt:]...)
	return out
}

func stampRuntimeVersion(dir string, manifest releaseManifest, check bool) (bool, error) {
	moduleDir, err := resolveModuleDir(dir)
	if err != nil {
		return false, err
	}
	var paths []string
	for _, rel := range []string{
		filepath.Join("internal", "cli", "version.go"),
		filepath.Join("internal", "cli", "root.go"),
	} {
		path := filepath.Join(moduleDir, rel)
		if exists(path) {
			paths = append(paths, path)
		}
	}
	matches, err := filepath.Glob(filepath.Join(moduleDir, "cmd", "*-pp-mcp", "main.go"))
	if err != nil {
		return false, err
	}
	paths = append(paths, matches...)

	changed := false
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return changed, err
		}
		next, touched := replaceVarString(data, "version", manifest.Version)
		if !touched {
			continue
		}
		if bytes.Contains(next, []byte(`var commit = "`)) {
			next, _ = replaceVarString(next, "commit", manifest.SourceCommit)
		}
		if bytes.Contains(next, []byte(`var date = "`)) {
			next, _ = replaceVarString(next, "date", manifest.ReleasedAt)
		}
		if changedFile, err := writeIfChanged(path, next, check); err != nil {
			return changed, err
		} else if changedFile {
			changed = true
		}
	}
	return changed, nil
}

func replaceVarString(data []byte, name, value string) ([]byte, bool) {
	re := regexp.MustCompile(fmt.Sprintf(varStringRe.String(), regexp.QuoteMeta(name)))
	replaced := false
	out := re.ReplaceAllFunc(data, func(match []byte) []byte {
		if replaced {
			return match
		}
		replaced = true
		parts := re.FindSubmatch(match)
		return []byte(string(parts[1]) + strconv.Quote(value))
	})
	return out, replaced
}

func writeIfChanged(path string, data []byte, check bool) (bool, error) {
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, data) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if check {
		return true, nil
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func slugFromDir(dir string) string {
	return filepath.Base(dir)
}

func cliKeyFromDir(dir string) string {
	return filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
}

// resolveModuleDir returns the directory that holds this CLI's go.mod.
//
// A catalog root that already has go.mod is that directory. Some CLIs cannot
// put the module there: a module path ending in /vN is invalid, so the module
// lives one directory down (library/ai/v0/vzero). When the catalog root has no
// go.mod, prefer the shared .goreleaser.yaml builds[].dir, and otherwise the
// single immediate child directory that contains a go.mod. Anything else stays
// the catalog root, which is what the stamper already did.
//
// tools/release-ledger/resolve-module-dir.sh is the same rule for the release
// workflows. TestResolveModuleDirMatchesShell keeps them together. This
// function stays in main.go so `go run ./tools/release-ledger/main.go` compiles it.
func resolveModuleDir(catalog string) (string, error) {
	if exists(filepath.Join(catalog, "go.mod")) {
		return catalog, nil
	}
	rel, err := goreleaserModuleRel(catalog)
	if err != nil {
		return "", err
	}
	if rel != "" {
		candidate := filepath.Join(catalog, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
	}
	rel, err = soleNestedModuleRel(catalog)
	if err != nil {
		return "", err
	}
	if rel != "" {
		return filepath.Join(catalog, rel), nil
	}
	return catalog, nil
}

func goreleaserModuleRel(catalog string) (string, error) {
	data, err := os.ReadFile(filepath.Join(catalog, ".goreleaser.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var dirs []string
	inBuilds := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indentOf(line) == 0 {
			inBuilds = trimmed == "builds:"
			continue
		}
		if !inBuilds {
			continue
		}
		item := trimmed
		if strings.HasPrefix(item, "- ") {
			item = strings.TrimSpace(strings.TrimPrefix(item, "- "))
		}
		key, val, ok := strings.Cut(item, ":")
		if !ok || strings.TrimSpace(key) != "dir" {
			continue
		}
		normalized, err := normalizeBuildDir(val)
		if err != nil {
			return "", fmt.Errorf("%s: %w", catalog, err)
		}
		if normalized != "" {
			dirs = append(dirs, normalized)
		}
	}
	unique := uniqueStrings(dirs)
	if len(unique) > 1 {
		return "", fmt.Errorf("%s: .goreleaser.yaml builds disagree on dir: %s", catalog, strings.Join(unique, ", "))
	}
	if len(unique) == 1 {
		return unique[0], nil
	}
	return "", nil
}

func normalizeBuildDir(raw string) (string, error) {
	val := strings.TrimSpace(raw)
	if hash := strings.Index(val, " #"); hash >= 0 {
		val = strings.TrimSpace(val[:hash])
	}
	val = strings.Trim(val, `"'`)
	val = strings.TrimSpace(val)
	if val == "" || val == "." || val == "./" {
		return "", nil
	}
	val = strings.TrimPrefix(val, "./")
	val = strings.TrimSuffix(val, "/")
	if val == "" || val == "." {
		return "", nil
	}
	if strings.Contains(val, "..") || strings.HasPrefix(val, "/") || strings.ContainsAny(val, " \t\\") {
		return "", fmt.Errorf("refusing goreleaser build dir %q", strings.TrimSpace(raw))
	}
	return val, nil
}

func soleNestedModuleRel(catalog string) (string, error) {
	entries, err := os.ReadDir(catalog)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, ".") || !entry.IsDir() {
			continue
		}
		if exists(filepath.Join(catalog, name, "go.mod")) {
			found = append(found, name)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	return "", nil
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
