// release-ledger maintains per-CLI release metadata for the published library.
//
// The library deliberately assigns these versions after a PR merges instead of
// asking contributors to edit 233 independent changelogs by hand. Versions use
// YYYY.M.N, where N is the release count for that CLI within the month.
//
// Usage:
//
//	go run ./tools/release-ledger/main.go --init-missing
//	go run ./tools/release-ledger/main.go --changed-from <sha> --changed-to <sha>
//	go run ./tools/release-ledger/main.go --check --changed-from <sha> --changed-to <sha>
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	initMissing  bool
	check        bool
	changedFrom  string
	changedTo    string
	releasedAt   time.Time
	sourceCommit string
	changeTitle  string
	changePR     int
	changeURL    string
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

	results, err := run(opts)
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
			log.Fatalf("release ledger drift detected:\n%s\nRun `go run ./tools/release-ledger/main.go` with the same scope and commit the result.", strings.Join(paths, "\n"))
		}
		fmt.Fprintln(os.Stderr, "release ledger is in sync")
		return
	}
	fmt.Fprintf(os.Stderr, "updated %d CLI release ledger entries\n", len(changed))
}

func parseFlags() (options, error) {
	var opts options
	var releasedAt string
	flag.BoolVar(&opts.initMissing, "init-missing", false, "initialize CLIs missing release metadata")
	flag.BoolVar(&opts.check, "check", false, "exit non-zero if the selected release ledger entries would change")
	flag.StringVar(&opts.changedFrom, "changed-from", "", "git ref to diff from when selecting changed CLIs")
	flag.StringVar(&opts.changedTo, "changed-to", "", "git ref to diff to when selecting changed CLIs")
	flag.StringVar(&releasedAt, "released-at", "", "release timestamp (RFC3339); defaults to now")
	flag.StringVar(&opts.sourceCommit, "source-commit", "", "source commit SHA; defaults to HEAD")
	flag.StringVar(&opts.changeTitle, "change-title", "", "human summary for the changelog entry")
	flag.IntVar(&opts.changePR, "change-pr", 0, "pull request number for the changelog entry")
	flag.StringVar(&opts.changeURL, "change-url", "", "pull request URL for the changelog entry")
	flag.Parse()

	if opts.initMissing && (opts.changedFrom != "" || opts.changedTo != "") {
		return opts, errors.New("--init-missing cannot be combined with --changed-from/--changed-to")
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

func changedCLIKeys(from, to string) (map[string]bool, error) {
	out := make(map[string]bool)
	diff, err := gitOutput("diff", "--name-only", "--diff-filter=ACMRT", from, to, "--", libraryDir)
	if err != nil {
		return out, err
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
		parts := strings.Split(name, "/")
		if len(parts) >= 3 && parts[0] == libraryDir {
			out[parts[1]+"/"+parts[2]] = true
		}
	}
	return out, nil
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
	var paths []string
	for _, rel := range []string{
		filepath.Join("internal", "cli", "version.go"),
		filepath.Join("internal", "cli", "root.go"),
	} {
		path := filepath.Join(dir, rel)
		if exists(path) {
			paths = append(paths, path)
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "cmd", "*-pp-mcp", "main.go"))
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
