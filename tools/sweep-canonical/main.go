// Command sweep-canonical applies the canonical published-library
// shape across every per-CLI entry in this repo. The shape is owned
// upstream by cli-printing-press's skill.md.tmpl and readme.md.tmpl
// (so fresh prints land correctly); this tool retrofits existing
// library/<cat>/<api>/SKILL.md and README.md files to match when an
// upstream template change ships and the existing entries need to
// catch up without a full regeneration.
//
// Originally introduced for the Hermes/OpenClaw frontmatter alignment
// (cli-printing-press
// docs/plans/2026-05-06-002-feat-hermes-openclaw-frontmatter-alignment-plan.md);
// has since broadened to also rewrite the SKILL.md Prerequisites
// section, the README ## Install section, and to insert the README
// Hermes/OpenClaw install blocks. The "frontmatter" name was retired
// because the tool now operates on body content too — its job is
// "apply canonical shape", not "patch frontmatter".
//
// What the tool patches in each library/<cat>/<api>/ directory:
//
// SKILL.md:
//   - Strips legacy OpenClaw env-var declarations (requires.env, envVars,
//     primaryEnv) from frontmatter.
//   - Adds the Hermes-recognized top-level fields (author, license) after
//     the description: line. Strips any pre-existing `version:` line —
//     it was emitted by an earlier sweep but tracked the Press version,
//     conflating generator with skill. Hermes lists `version` as
//     optional; omission is the honest move until per-CLI release
//     versions can be stamped at library CI time from goreleaser tags.
//   - Moves the existing `## CLI Installation` section to immediately
//     after the H1 and rewrites it as `## Prerequisites: Install the
//     CLI` with imperative "you must verify ... do not proceed" wording.
//     For CLIs that lack a `## CLI Installation` section entirely, the
//     Prerequisites section is constructed from manifest data instead.
//
// README.md:
//   - Rewrites the `## Install` section (with its `### Binary` /
//     `### Go` subsections) to lead with the `npx -y
//     @mvanhorn/printing-press-library install <api>` installer (CLI + agent
//     skill), with `--cli-only` and a Go fallback below. Mirrors the
//     upstream readme.md.tmpl change.
//   - Inserts `## Install via Hermes` and `## Install via OpenClaw`
//     sections, anchored on the <!-- pp-hermes-install-anchor -->
//     comment when present, or via a fallback chain (Use with Claude
//     Desktop -> Use with Claude Code -> ## Install -> EOF) for
//     legacy READMEs.
//
// Idempotent: running twice produces zero textual diff on the second run.
// Snapshot-restore: if any per-CLI patch fails, all touched files for
// that CLI are restored from in-memory snapshots before moving on. The
// rest of the sweep continues.
//
// Once every library entry has been swept and the upstream templates
// match, the regen workflow takes over for ongoing changes — fresh
// prints from cli-printing-press already produce canonical-shape
// output, so this tool is only invoked when an upstream template
// change ships and existing entries need to catch up.
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type manifest struct {
	CLIName     string `json:"cli_name"`
	APIName     string `json:"api_name"`
	OwnerName   string `json:"owner_name"`
	Owner       string `json:"owner"`
	Printer     string `json:"printer"`
	PrinterName string `json:"printer_name"`
}

// person is one credited human in the creator + contributors attribution
// model. Mirrors the generator's spec.Person: Handle is the slug-safe GitHub
// @handle that drives path/regex surfaces (copyright header recovery, byline
// links); Name is the prose display name (README byline parenthetical, NOTICE,
// SKILL author:). The JSON tags match the generator's emission so a swept
// manifest is byte-identical to a fresh print for the same identity.
type person struct {
	Handle string `json:"handle,omitempty"`
	Name   string `json:"name,omitempty"`
}

// resolveCreator derives the permanent creator from a legacy manifest, matching
// the generator's resolution (New() seeds creator from printer, then owner):
//
//   - Handle ← printer → owner.
//   - Name   ← curated cliAuthorByAPIName → printer_name → owner_name → handle.
//
// The curated map is consulted first for Name because it is the source of truth
// for the existing published CLIs' display names (see AGENTS.md); legacy
// printer_name/owner_name are often handle-shaped for newer entries, and the
// handle is the last-resort so a byline link always has a label.
func resolveCreator(mf manifest) person {
	handle := mf.Printer
	if handle == "" {
		handle = mf.Owner
	}
	name := cliAuthorByAPIName[mf.APIName]
	if name == "" {
		name = mf.PrinterName
	}
	if name == "" {
		name = mf.OwnerName
	}
	if name == "" {
		name = handle
	}
	return person{Handle: handle, Name: name}
}

// cliAuthorByAPIName is the canonical author display name for every
// existing per-CLI library entry, keyed by the api_name (the directory
// basename: dominos, linear, etc.). The entries were derived from
// each CLI's `// Copyright YYYY <slug>.` header where present, with
// per-CLI corrections applied for the cases where the slug doesn't
// reflect actual authorship — generator-fallback "user" headers
// (5 CLIs originally generated by Cathryn before git config was set),
// missing copyright headers (2 legacy CLIs), and one slug-vs-actual
// mismatch (espn — copyright slug "trevin-chow" but actually Matt's
// work).
//
// Entries here are the source of truth for the author: field that
// lands in published library/<cat>/<api>/SKILL.md and the byte-
// identical cli-skills/pp-<api>/SKILL.md mirror. The operator's
// git config is consulted only as a last-resort fallback for new
// CLIs not in this map.
var cliAuthorByAPIName = map[string]string{
	"agent-capture":   "Matt Van Horn",
	"ahrefs":          "Cathryn Lavery",
	"airbnb":          "Matt Van Horn",
	"allrecipes":      "Trevin Chow",
	"amazon-ads":      "Cathryn Lavery",
	"amazon-seller":   "Cathryn Lavery",
	"apartments":      "rderwin",
	"archive-is":      "Matt Van Horn",
	"cal-com":         "Trevin Chow",
	"coingecko":       "Hiten Shah",
	"company-goat":    "Trevin Chow",
	"contact-goat":    "Matt Van Horn",
	"craigslist":      "Trevin Chow",
	"docker-hub":      "Hiten Shah",
	"dominos":         "Matt Van Horn",
	"dub":             "Trevin Chow",
	"ebay":            "Matt Van Horn",
	"espn":            "Matt Van Horn",
	"fedex":           "Trevin Chow",
	"firecrawl":       "Hiten Shah",
	"flight-goat":     "Matt Van Horn",
	"food52":          "Trevin Chow",
	"google-photos":   "Cathryn Lavery",
	"hackernews":      "Trevin Chow",
	"instacart":       "Matt Van Horn",
	"kalshi":          "Trevin Chow",
	"klaviyo":         "Cathryn Lavery",
	"linear":          "Matt Van Horn",
	"mercury":         "Cathryn Lavery",
	"movie-goat":      "Trevin Chow",
	"nvd":             "Hiten Shah",
	"open-meteo":      "Trevin Chow",
	"pagliacci":       "Trevin Chow",
	"pokeapi":         "Hiten Shah",
	"postman-explore": "Trevin Chow",
	"producthunt":     "Trevin Chow",
	"pypi":            "Hiten Shah",
	"recipe-goat":     "Trevin Chow",
	"render":          "Giuliano Giacaglia",
	"scrape-creators": "Adrian Horning",
	"seats-aero":      "Cathryn Lavery",
	"sentry":          "Cathryn Lavery",
	"shopify":         "Cathryn Lavery",
	"slack":           "Matt Van Horn",
	"steam-web":       "Trevin Chow",
	"tiktok-shop":     "Cathryn Lavery",
	"trigger-dev":     "Matt Van Horn",
	"weather-goat":    "Trevin Chow",
	"wikipedia":       "Hiten Shah",
	"yahoo-finance":   "Trevin Chow",
}

func main() {
	libraryRoot := "library"
	if v := os.Getenv("SWEEP_LIBRARY_ROOT"); v != "" {
		libraryRoot = v
	}

	// -readme-only / SWEEP_README_ONLY: skip SKILL.md patching entirely.
	// Use this when running the sweep from a workspace whose
	// `git config user.name` differs from the canonical maintainer
	// identity, or when iterating on a README-only template change and
	// you don't want skill churn in the diff. The SKILL.md path is also
	// defended in-place (existing author values are preserved unless
	// they're the placeholder "user"), but this flag is the belt-and-
	// suspenders option when you don't want SKILL.md touched at all.
	readmeOnly := false
	// -backfill-contributors: opt-in. Computes each CLI's contributors from
	// git history (denylist-filtered, creator-excluded), prints a
	// human-reviewable per-CLI table, then writes contributors[] into the
	// manifests / NOTICE / README byline. The default run (flag off) writes
	// creator-only — the safe mechanical migration is kept separate from the
	// judgment-heavy attribution step (see the prior misattribution scar).
	backfill := false
	// -attribution-only: apply ONLY the creator + contributors surfaces
	// (manifest, .go copyright headers, NOTICE, README byline) and skip the
	// SKILL.md and README install/Hermes/OpenClaw shape transforms. Use this
	// for a focused attribution migration so unrelated docs-shape drift doesn't
	// ride along in the diff.
	attributionOnly := false
	for _, a := range os.Args[1:] {
		switch a {
		case "-readme-only", "--readme-only":
			readmeOnly = true
		case "-backfill-contributors", "--backfill-contributors":
			backfill = true
		case "-attribution-only", "--attribution-only":
			attributionOnly = true
		}
	}
	if !readmeOnly && strings.EqualFold(os.Getenv("SWEEP_README_ONLY"), "1") {
		readmeOnly = true
	}

	ownerName := strings.TrimSpace(os.Getenv("SWEEP_OWNER_NAME"))
	if ownerName == "" {
		out, err := exec.Command("git", "config", "user.name").Output()
		if err == nil {
			ownerName = strings.TrimSpace(string(out))
		}
	}
	if !readmeOnly && ownerName == "" {
		log.Fatalf("could not resolve owner display name: set SWEEP_OWNER_NAME, or `git config user.name`. " +
			"the value lands as `author:` in the published library/<cat>/<api>/SKILL.md frontmatter " +
			"only when the existing author is missing or is the placeholder \"user\". " +
			"pass -readme-only if you don't want SKILL.md touched at all.")
	}

	cliDirs, err := findCLIDirs(libraryRoot)
	if err != nil {
		log.Fatalf("walking %s: %v", libraryRoot, err)
	}
	if len(cliDirs) == 0 {
		log.Fatalf("no per-CLI directories found under %s", libraryRoot)
	}

	if readmeOnly {
		fmt.Println("Running in -readme-only mode: SKILL.md files will not be touched.")
	}
	if attributionOnly {
		fmt.Println("Running in -attribution-only mode: only manifest/header/NOTICE/byline are patched; SKILL.md and README install/Hermes shape are left untouched.")
	}

	// Contributor backfill (opt-in): compute + surface the per-CLI table
	// BEFORE any mutation, so the attribution can be reviewed.
	contribByDir := map[string][]person{}
	if backfill {
		unresolvedByDir := map[string][]string{}
		for _, dir := range cliDirs {
			mf, err := readManifestForDir(dir)
			if err != nil {
				continue
			}
			res := backfillContributors(dir, resolveCreator(mf))
			contribByDir[dir] = res.contributors
			if len(res.unresolved) > 0 {
				unresolvedByDir[dir] = res.unresolved
			}
		}
		printContributorTable(cliDirs, contribByDir, unresolvedByDir)
	}

	var processed, skipped, errored int
	for _, dir := range cliDirs {
		status, err := sweepCLI(dir, ownerName, readmeOnly, attributionOnly, contribByDir[dir])
		switch {
		case err != nil:
			fmt.Printf("  ERROR %s: %v\n", dir, err)
			errored++
		case status == statusUnchanged:
			skipped++
		default:
			fmt.Printf("  SWEPT %s (%s)\n", dir, status)
			processed++
		}
	}

	fmt.Printf("\nSweep complete: %d patched, %d already up-to-date, %d errored\n", processed, skipped, errored)
	if errored > 0 {
		os.Exit(1)
	}
}

// findCLIDirs returns library/<cat>/<api>/ directories in deterministic order.
func findCLIDirs(libraryRoot string) ([]string, error) {
	cats, err := os.ReadDir(libraryRoot)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, cat := range cats {
		if !cat.IsDir() {
			continue
		}
		catPath := filepath.Join(libraryRoot, cat.Name())
		apis, err := os.ReadDir(catPath)
		if err != nil {
			return nil, err
		}
		for _, api := range apis {
			if !api.IsDir() {
				continue
			}
			dirs = append(dirs, filepath.Join(catPath, api.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// sweepStatus is the per-CLI sweep outcome. statusUnchanged means every
// surface was already at canonical shape; any other value is a human-readable
// summary of what changed (e.g. "42 files").
type sweepStatus string

const statusUnchanged sweepStatus = "unchanged"

const minimumGoVersion = "Go 1.26.5 or newer"

// sweepCLI applies the canonical shape to one library/<cat>/<api>/. It patches
// SKILL.md (canonical docs shape) and README.md (install + alternate-install
// blocks + byline), and migrates the attribution surfaces to the creator +
// contributors model: the manifest, every generated .go copyright header, the
// NOTICE, and the README byline. All pending writes are collected first; on any
// write failure every file already written for this CLI is rolled back from its
// in-memory snapshot before returning.
//
// readmeOnly skips SKILL.md patching entirely (see main()). The attribution
// surfaces derive entirely from the manifest, not the operator's identity, so
// they run regardless of readmeOnly.
//
// contributors is the per-CLI list computed by -backfill-contributors (empty in
// the default run, which writes creator-only).
//
// attributionOnly applies only the attribution surfaces (manifest, headers,
// NOTICE, README byline) and skips the SKILL.md and README install/Hermes shape
// transforms — see main().
func sweepCLI(cliDir, ownerName string, readmeOnly, attributionOnly bool, contributors []person) (sweepStatus, error) {
	skillPath := filepath.Join(cliDir, "SKILL.md")
	readmePath := filepath.Join(cliDir, "README.md")
	manifestPath := filepath.Join(cliDir, ".printing-press.json")
	noticePath := filepath.Join(cliDir, "NOTICE")

	mfData, err := os.ReadFile(manifestPath)
	if err != nil {
		return statusUnchanged, fmt.Errorf("read manifest: %w", err)
	}
	var mf manifest
	if err := json.Unmarshal(mfData, &mf); err != nil {
		return statusUnchanged, fmt.Errorf("parse manifest: %w", err)
	}
	if mf.CLIName == "" {
		return statusUnchanged, fmt.Errorf("manifest missing cli_name")
	}

	// Authority for category: directory path. The manifest's category
	// field is omitempty and missing in many legacy manifests, so
	// trusting the on-disk location is more reliable.
	category := filepath.Base(filepath.Dir(cliDir))
	if category == "" {
		category = "other"
	}

	// Authorship resolution — produces the value used ONLY when the
	// existing SKILL.md author is missing or the placeholder "user"
	// (see ensureFrontmatterTopLevelFields). Real existing authors are
	// preserved unconditionally.
	//
	//  1. cliAuthorByAPIName — the curated per-CLI mapping; the source
	//     of truth for the existing published CLIs. Honors actual
	//     authorship rather than guessing from git history.
	//  2. Manifest's owner_name — set by future fresh prints. Lets a
	//     regen preserve attribution.
	//  3. Operator's git config user.name — last-resort fallback for
	//     a new CLI added to the library without an entry in the map
	//     above.
	authorName := cliAuthorByAPIName[mf.APIName]
	if authorName == "" {
		authorName = mf.OwnerName
	}
	if authorName == "" {
		authorName = ownerName
	}

	// The permanent creator drives every attribution surface. Resolved from
	// the legacy manifest fields (printer/owner + curated name).
	creator := resolveCreator(mf)

	var edits []fileEdit

	// SKILL.md (canonical docs shape) — skipped in readme-only and
	// attribution-only modes.
	if !readmeOnly && !attributionOnly {
		skillBefore, err := os.ReadFile(skillPath)
		if err != nil {
			return statusUnchanged, fmt.Errorf("read SKILL.md: %w", err)
		}
		skillAfter, err := patchSkill(string(skillBefore), patchSkillCtx{
			CLIName:    mf.CLIName,
			APIName:    mf.APIName,
			Category:   category,
			AuthorName: authorName,
		})
		if err != nil {
			return statusUnchanged, fmt.Errorf("patch SKILL.md: %w", err)
		}
		if skillAfter != string(skillBefore) {
			edits = append(edits, fileEdit{skillPath, skillBefore, []byte(skillAfter)})
		}
	}

	// README.md (install/alternate-install/byline).
	readmeBefore, err := os.ReadFile(readmePath)
	if err != nil {
		return statusUnchanged, fmt.Errorf("read README.md: %w", err)
	}
	readmeAfter, err := patchReadme(string(readmeBefore), patchReadmeCtx{
		CLIName:      mf.CLIName,
		APIName:      mf.APIName,
		Category:     category,
		Creator:      creator,
		Contributors: contributors,
		BylineOnly:   attributionOnly,
	})
	if err != nil {
		return statusUnchanged, fmt.Errorf("patch README.md: %w", err)
	}
	if readmeAfter != string(readmeBefore) {
		edits = append(edits, fileEdit{readmePath, readmeBefore, []byte(readmeAfter)})
	}

	// Manifest: insert creator (+ contributors). Validate it stays parseable
	// JSON before queuing the write.
	if manifestAfter, changed := patchManifest(string(mfData), creator, contributors); changed {
		if !json.Valid([]byte(manifestAfter)) {
			return statusUnchanged, fmt.Errorf("patched manifest is not valid JSON")
		}
		edits = append(edits, fileEdit{manifestPath, mfData, []byte(manifestAfter)})
	}

	// NOTICE (not every CLI ships one).
	if noticeBefore, err := os.ReadFile(noticePath); err == nil {
		if noticeAfter, changed := patchNOTICE(string(noticeBefore), creator, contributors); changed {
			edits = append(edits, fileEdit{noticePath, noticeBefore, []byte(noticeAfter)})
		}
	} else if !os.IsNotExist(err) {
		return statusUnchanged, fmt.Errorf("read NOTICE: %w", err)
	}

	// Copyright headers across every generated .go (display name + constant
	// " and contributors" suffix).
	headerEdits, err := patchCopyrightHeaders(cliDir, creator.Name)
	if err != nil {
		return statusUnchanged, fmt.Errorf("scan copyright headers: %w", err)
	}
	edits = append(edits, headerEdits...)

	if len(edits) == 0 {
		return statusUnchanged, nil
	}

	// Write all edits, rolling back already-written files on any failure.
	var written []fileEdit
	for _, e := range edits {
		if err := os.WriteFile(e.path, e.after, 0o644); err != nil {
			for _, w := range written {
				if rerr := os.WriteFile(w.path, w.before, 0o644); rerr != nil {
					fmt.Printf("    WARN restore %s failed: %v\n", w.path, rerr)
				}
			}
			return statusUnchanged, fmt.Errorf("write %s: %w", e.path, err)
		}
		written = append(written, e)
	}

	return sweepStatus(fmt.Sprintf("%d files", len(edits))), nil
}

type patchSkillCtx struct {
	CLIName           string // e.g. "shopify-pp-cli"
	APIName           string // e.g. "shopify"
	Category          string // e.g. "commerce"
	AuthorName        string // display name, e.g. "Trevin Chow"
	FillMissingAuthor bool   // opt in only for dedicated attribution sweeps
}

// patchSkill applies the canonical Hermes/OpenClaw shape to a SKILL.md
// body. Idempotent: if the body already has the canonical Prerequisites
// section near the top and lacks the legacy OpenClaw env declarations,
// it's returned unchanged.
func patchSkill(body string, ctx patchSkillCtx) (string, error) {
	body = patchSkillFrontmatter(body, ctx)
	body = patchSkillPrerequisites(body, ctx)
	body = patchSkillReferences(body, ctx.CLIName)
	return body, nil
}

// patchSkillFrontmatter rewrites the YAML frontmatter region:
//   - Strips `      env: ...` line under requires (4 shapes: empty inline,
//     single inline, block-style, absent).
//   - Strips entire `    envVars:` block including all indented children.
//   - Strips `    primaryEnv: ...` line.
//   - Adds `author`, `license` top-level fields after the `description:`
//     line. Strips any pre-existing `version:` line — see top-of-file
//     comment for why version is omitted.
//
// Body content (after the closing ---) is byte-preserved.
func patchSkillFrontmatter(body string, ctx patchSkillCtx) string {
	const fence = "---\n"
	if !strings.HasPrefix(body, fence) {
		return body
	}
	end := strings.Index(body[len(fence):], "\n"+fence)
	if end < 0 {
		return body
	}
	frontmatter := body[len(fence) : len(fence)+end+1] // include trailing \n
	rest := body[len(fence)+end+len(fence)+1:]         // after second ---\n

	frontmatter = stripFrontmatterLegacyEnvBlocks(frontmatter)
	frontmatter = ensureFrontmatterTopLevelFields(frontmatter, ctx)

	return fence + frontmatter + fence + rest
}

// stripFrontmatterLegacyEnvBlocks removes the legacy OpenClaw env
// declarations from a frontmatter string. Handles all four observed
// shapes:
//   - `      env: []`             (empty inline list)
//   - `      env: ["FOO"]`        (single inline list)
//   - `      env:\n        - FOO` (block-style list with indented items)
//   - `    envVars:\n      - ...` (multi-line envVars block)
//   - `    primaryEnv: VALUE`     (legacy single-key field)
func stripFrontmatterLegacyEnvBlocks(fm string) string {
	lines := strings.Split(fm, "\n")
	var out []string
	skipUntilDedent := -1 // base indent level of a multi-line block being skipped; -1 when not skipping
	for _, line := range lines {
		if skipUntilDedent >= 0 {
			indent := leadingSpaces(line)
			// Skip continuation lines: blank lines or anything more-indented
			// than the block's base. The block ends at the first non-blank
			// line whose indent is <= the base.
			if strings.TrimSpace(line) == "" || indent > skipUntilDedent {
				continue
			}
			skipUntilDedent = -1
			// fall through to evaluate this line normally
		}

		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)

		// `      env: ...` under requires (indent 6 in the canonical shape).
		// Catches both inline list ([] or ["FOO"]) and block-style header
		// (just `env:`).
		if indent == 6 && (strings.HasPrefix(trimmed, "env: ") || trimmed == "env:") {
			if trimmed == "env:" {
				// Block-style: skip the indented list items that follow.
				skipUntilDedent = indent
			}
			continue
		}

		// `    envVars:` (indent 4) — strip header AND all indented content
		if indent == 4 && trimmed == "envVars:" {
			skipUntilDedent = indent
			continue
		}

		// `    primaryEnv: VALUE` — single line, no continuation
		if indent == 4 && strings.HasPrefix(trimmed, "primaryEnv:") {
			continue
		}

		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func leadingSpaces(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			return i
		}
	}
	return len(s)
}

// ensureFrontmatterTopLevelFields normalizes `author` and `license` to
// the canonical "after description" position. Both are stripped first
// and re-inserted as a contiguous block, so:
//
//   - Author preserves any existing value already in the frontmatter,
//     including the old generator-fallback placeholder "user". Missing
//     authors are filled only when ctx.FillMissingAuthor is true, so a
//     docs-shape sweep cannot accidentally bundle attribution changes
//     with unrelated README/SKILL surface edits.
//   - License is always "Apache-2.0" — the constant for every printed
//     CLI per LICENSE.tmpl.
//
// Also strips any pre-existing `version:` line (legacy from an earlier
// sweep that emitted the Press version) without re-emitting one. See
// top-of-file comment for the rationale.
//
// Idempotent in the second-run-zero-diff sense — running with the
// same ctx produces the same output.
func ensureFrontmatterTopLevelFields(fm string, ctx patchSkillCtx) string {
	existingAuthor := extractTopLevelFieldValue(fm, "author")
	hasAuthor := topLevelFieldRe("author").FindStringIndex(fm) != nil

	fm = stripTopLevelField(fm, "version")
	fm = stripTopLevelField(fm, "author")
	fm = stripTopLevelField(fm, "license")

	var b strings.Builder
	if hasAuthor {
		fmt.Fprintf(&b, "author: %q\n", existingAuthor)
	} else if ctx.FillMissingAuthor && ctx.AuthorName != "" {
		fmt.Fprintf(&b, "author: %q\n", ctx.AuthorName)
	}
	fmt.Fprintf(&b, "license: %q\n", "Apache-2.0")
	block := b.String()

	descRe := regexp.MustCompile(`(?m)^description: ".*"\n`)
	return descRe.ReplaceAllStringFunc(fm, func(match string) string {
		return match + block
	})
}

// stripTopLevelField removes the line `<name>: "..."\n` from a
// frontmatter string when present at the top level.
func stripTopLevelField(fm, name string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `: ".*"\n`)
	return re.ReplaceAllString(fm, "")
}

// extractTopLevelFieldValue returns the quoted value of a top-level
// `<name>: "..."` line in the frontmatter, or "" when absent.
func extractTopLevelFieldValue(fm, name string) string {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `: "([^"]*)"$`)
	m := re.FindStringSubmatch(fm)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// topLevelFieldRe returns a regexp matching `<name>:` at the start of a
// line — i.e. a top-level (non-indented) frontmatter field.
func topLevelFieldRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `:\s`)
}

// patchSkillPrerequisites moves the existing `## CLI Installation`
// section to immediately after the H1 and renames it. For SKILL.md
// files that lack `## CLI Installation` entirely (4 in the live
// library), constructs the section from manifest data. If a
// previous sweep already inserted a `## Prerequisites: Install the
// CLI` section, that prior section is removed and the canonical
// content is re-emitted — so install-command updates (e.g., a
// switch from `go install` to `npx ... install --cli-only`)
// propagate across re-sweeps without manual intervention.
//
// Idempotent in the second-run-zero-diff sense: running with the
// same ctx produces the same Prerequisites content.
func patchSkillPrerequisites(body string, ctx patchSkillCtx) string {
	// Remove an existing Prerequisites section (from a prior sweep) so
	// the canonical content can be re-emitted with any updates.
	body = removePrerequisitesSection(body)

	prereq := buildPrerequisitesSection(ctx)

	// Try to remove the existing `## CLI Installation` section.
	body, removedCLIInstall := removeCLIInstallationSection(body)

	// Insert Prerequisites right after the H1 line. If we couldn't find
	// an H1, insert at the very top after the closing frontmatter ---.
	body = insertAfterH1(body, prereq)

	// If we removed the existing CLI Installation section, also update
	// any remaining references (the Direct Use section's "see CLI
	// Installation above" hint is the canonical one, but other prose
	// may reference it too).
	if removedCLIInstall {
		body = strings.ReplaceAll(body, "(see CLI Installation above)",
			"(see Prerequisites at the top of this skill)")
		// The Argument Parsing rule uses the phrase "CLI installation"
		// in routing guidance — update to point at Prerequisites.
		body = strings.ReplaceAll(body,
			"otherwise → CLI installation",
			"otherwise → see Prerequisites above")
	}

	return body
}

// removePrerequisitesSection strips an existing `## Prerequisites:
// Install the CLI` section (heading + body up to the next `## `
// heading or EOF) so the sweep can re-emit canonical content. Used
// to make install-command updates (e.g., switching the install line
// from `go install` to `npx ... install --cli-only`) propagate
// across re-sweeps.
func removePrerequisitesSection(body string) string {
	const heading = "## Prerequisites: Install the CLI"
	idx := strings.Index(body, heading)
	if idx < 0 {
		return body
	}
	rest := body[idx+len(heading):]
	nextIdx := strings.Index(rest, "\n## ")
	var sectionEnd int
	if nextIdx < 0 {
		sectionEnd = len(body)
	} else {
		sectionEnd = idx + len(heading) + nextIdx + 1
	}
	start := idx
	for start > 0 && body[start-1] == '\n' {
		start--
		if start > 0 && body[start-1] != '\n' {
			break
		}
	}
	return body[:start+1] + body[sectionEnd:]
}

func buildPrerequisitesSection(ctx patchSkillCtx) string {
	module := fmt.Sprintf(
		"github.com/mvanhorn/printing-press-library/library/%s/%s/cmd/%s",
		ctx.Category, ctx.APIName, ctx.CLIName,
	)
	return fmt.Sprintf(`## Prerequisites: Install the CLI

This skill drives the `+"`%s`"+` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `+"`$HOME/.local/bin`"+` on macOS/Linux and `+"`%%LOCALAPPDATA%%\\Programs\\PrintingPress\\bin`"+` on Windows:
   `+"```bash"+`
   npx -y @mvanhorn/printing-press-library install %s --cli-only
   `+"```"+`
2. Verify: `+"`%s --version`"+`
3. Ensure the reported install directory is on `+"`$PATH`"+` for the agent/runtime that will invoke this skill.

If the `+"`npx`"+` install fails (no Node, offline, etc.), fall back to a direct Go install (requires %s):

`+"```bash"+`
go install %s@latest
`+"```"+`

If `+"`--version`"+` reports "command not found" after install, the runtime cannot see the binary directory on `+"`$PATH`"+`. Do not proceed with skill commands until verification succeeds.

`, ctx.CLIName, ctx.APIName, ctx.CLIName, minimumGoVersion, module)
}

// removeCLIInstallationSection strips the existing `## CLI Installation`
// section (heading + body up to the next `## ` heading or EOF). Returns
// the modified body and a bool indicating whether the section was found.
func removeCLIInstallationSection(body string) (string, bool) {
	const heading = "## CLI Installation"
	idx := strings.Index(body, heading)
	if idx < 0 {
		return body, false
	}
	// Find the start of the next `## ` heading after this section.
	rest := body[idx+len(heading):]
	nextIdx := strings.Index(rest, "\n## ")
	var sectionEnd int
	if nextIdx < 0 {
		sectionEnd = len(body)
	} else {
		sectionEnd = idx + len(heading) + nextIdx + 1 // include the \n before next heading
	}
	// Also strip the leading blank line(s) before the heading so removal
	// doesn't leave a double-blank gap.
	start := idx
	for start > 0 && body[start-1] == '\n' {
		start--
		if start > 0 && body[start-1] != '\n' {
			break
		}
	}
	return body[:start+1] + body[sectionEnd:], true
}

// insertAfterH1 inserts content right after the first `# ` heading line.
// If no H1 is found, inserts at the start of the body.
func insertAfterH1(body, content string) string {
	// Find first `# ` line (not `## `).
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			// Insert two lines after the H1: one blank, then content.
			head := strings.Join(lines[:i+1], "\n")
			tail := strings.Join(lines[i+1:], "\n")
			return head + "\n\n" + content + tail
		}
	}
	return content + body
}

// patchSkillReferences fixes any stale references to the old CLI
// Installation heading in body prose that wasn't caught by the
// removeCLIInstallationSection-conditional rewriter (e.g. when the
// section was already removed by an earlier sweep run but a reference
// lingered).
func patchSkillReferences(body string, cliName string) string {
	body = strings.ReplaceAll(body, "(see CLI Installation above)",
		"(see Prerequisites at the top of this skill)")
	body = strings.ReplaceAll(body,
		"otherwise → CLI installation",
		"otherwise → see Prerequisites above")
	return body
}

type patchReadmeCtx struct {
	CLIName      string
	APIName      string
	Category     string
	Creator      person   // drives the `Created by` byline (empty Handle = skip byline)
	Contributors []person // drives the optional `Contributors:` byline line
	BylineOnly   bool     // attribution-only mode: patch the byline, skip install/Hermes shape
}

// patchReadme applies the canonical README shape:
//  1. Rewrites the `## Install` section to lead with the `npx -y
//     @mvanhorn/printing-press-library install <api>` installer (CLI + agent
//     skill), with `--cli-only` and a Go fallback below. Mirrors the
//     upstream readme.md.tmpl shape.
//  2. Inserts the `## Install via Hermes` and `## Install via
//     OpenClaw` sections.
//
// Both steps are idempotent in the second-run-zero-diff sense.
func patchReadme(body string, ctx patchReadmeCtx) (string, error) {
	if !ctx.BylineOnly {
		body = patchReadmeInstall(body, ctx)
		body = patchReadmeHermesOpenClaw(body, ctx)
	}
	body = patchReadmeByline(body, ctx.Creator, ctx.Contributors)
	return body, nil
}

// patchReadmeInstall finds the `## Install` heading and rewrites its
// body up to the next `## ` heading with the canonical block (npx
// default → `--cli-only` → Go fallback → pre-built binary).
//
// Idempotent: running with the same ctx produces the same output. We
// achieve this by always re-emitting the canonical block — running
// against a body that already has the canonical content produces the
// same canonical content.
//
// READMEs without a `## Install` section (2 in the live library:
// agent-capture and the contact-goat manuscript copy) are left
// untouched. Adding an Install section to those is out of scope —
// they have hand-shaped install guidance that doesn't fit the
// template.
func patchReadmeInstall(body string, ctx patchReadmeCtx) string {
	// Match the literal H2 with trailing newline so we don't pick up
	// `## Install via Hermes`, `## Installation`, etc.
	const heading = "## Install\n"
	idx := strings.Index(body, heading)
	if idx < 0 {
		return body
	}

	// Find the next H2 heading after this one — that's where the
	// Install section ends. Match `\n## ` to avoid matching `### `.
	tail := body[idx+len(heading):]
	nextIdx := strings.Index(tail, "\n## ")
	var sectionEnd int
	if nextIdx < 0 {
		sectionEnd = len(body)
	} else {
		sectionEnd = idx + len(heading) + nextIdx + 1 // include the \n before next heading
	}

	canonical := buildReadmeInstallSection(ctx)
	return body[:idx] + canonical + body[sectionEnd:]
}

func buildReadmeInstallSection(ctx patchReadmeCtx) string {
	module := fmt.Sprintf(
		"github.com/mvanhorn/printing-press-library/library/%s/%s/cmd/%s",
		ctx.Category, ctx.APIName, ctx.CLIName,
	)
	return fmt.Sprintf(`## Install

The recommended path installs both the `+"`%s`"+` binary and the `+"`pp-%s`"+` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`+"`skills`"+`](https://github.com/vercel-labs/skills) CLI) in one shot:

`+"```bash"+`
npx -y @mvanhorn/printing-press-library install %s
`+"```"+`

For CLI only (no skill):

`+"```bash"+`
npx -y @mvanhorn/printing-press-library install %s --cli-only
`+"```"+`

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

`+"```bash"+`
npx -y @mvanhorn/printing-press-library install %s --skill-only
`+"```"+`

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`+"`skills`"+`](https://github.com/vercel-labs/skills) CLI):

`+"```bash"+`
npx -y @mvanhorn/printing-press-library install %s --agent claude-code
npx -y @mvanhorn/printing-press-library install %s --agent claude-code --agent codex
`+"```"+`

### Without Node (Go fallback)

If `+"`npx`"+` isn't available (no Node, offline), install the CLI directly via Go (requires %s):

`+"```bash"+`
go install %s@latest
`+"```"+`

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/%s-current). On macOS, clear the Gatekeeper quarantine: `+"`xattr -d com.apple.quarantine <binary>`"+`. On Unix, mark it executable: `+"`chmod +x <binary>`"+`.

`,
		ctx.CLIName, // binary name in headline
		ctx.APIName, // skill name in headline
		ctx.APIName, // bundled install slug
		ctx.APIName, // --cli-only slug
		ctx.APIName, // --skill-only slug
		ctx.APIName, // --agent single slug
		ctx.APIName, // --agent multi slug
		minimumGoVersion,
		module,      // Go fallback module
		ctx.APIName, // pre-built release tag
	)
}

// patchReadmeHermesOpenClaw enforces the canonical position and
// naming for the alternate install paths grouped at the top of the
// README:
//
//  1. Strips any existing Hermes/OpenClaw sections and the
//     pp-hermes-install-anchor comment from wherever they currently
//     are. Handles both legacy "Install via X" naming and current
//     "Install for X" naming, so the function is forward- and
//     backward-compatible across template versions.
//  2. Strips the legacy `## Use with Claude Code` section entirely —
//     its skill-install command is now redundant with the
//     `--skill-only` and `--agent` options documented in the canonical
//     `## Install` block, and its MCP `<details>` subsection drifted
//     in placement across CLIs without adding value.
//  3. Extracts the `## Use with Claude Desktop` section (when present —
//     not every CLI ships an MCPB bundle) so it can be re-inserted at
//     the canonical position alongside Hermes/OpenClaw. Claude Desktop
//     is a separate install path because the MCPB bundle is its own
//     thing — parallel to Hermes and OpenClaw, not duplicative of
//     `## Install`.
//  4. Re-inserts the canonical install paths (preceded by the anchor
//     comment) immediately after the `## Install` section ends:
//     Hermes → OpenClaw → Claude Desktop (when present). All install
//     paths now sit grouped at the top of the README.
//
// Idempotent: running with body that already has the canonical shape
// strips and re-inserts identical content, producing zero diff. The
// Claude Desktop section is moved verbatim — the sweep does not
// reformat its content, only its position.
//
// READMEs without a `## Install` heading (only agent-capture in the
// live library) are left unchanged — their hand-shaped install
// guidance doesn't fit the template, and dropping the alternate-install
// blocks at an arbitrary position would be worse than leaving them
// off entirely.
func patchReadmeHermesOpenClaw(body string, ctx patchReadmeCtx) string {
	// If there's no ## Install heading to anchor on, don't touch the
	// README — stripping existing alternate-install blocks without
	// being able to re-insert them would silently delete install
	// instructions. Only agent-capture among the live CLIs falls in
	// this branch; its README has hand-shaped install guidance that
	// doesn't fit the template.
	const installHeading = "## Install\n"
	if !strings.Contains(body, installHeading) {
		return body
	}

	// Extract the Claude Desktop section verbatim (heading + body up to
	// the next H2) so we can re-insert it at the canonical position. If
	// the CLI doesn't ship an MCPB bundle, there's no section to move
	// and this stays empty — the canonical block omits the Claude
	// Desktop line entirely.
	claudeDesktopSection := extractH2Section(body, "## Use with Claude Desktop")
	// Strip any anchor comment that fell inside the extracted section
	// (e.g., trigger-dev's pre-PR layout had the anchor sitting between
	// the Claude Desktop body and the next H2). Without this, the
	// stale anchor rides along to the canonical position and produces
	// a duplicate alongside the canonical anchor we re-insert below.
	// Both are stripped from the rest of `body` later, but the
	// extracted section needs its own pass.
	claudeDesktopSection = strings.ReplaceAll(claudeDesktopSection, "<!-- pp-hermes-install-anchor -->\n", "")
	claudeDesktopSection = strings.ReplaceAll(claudeDesktopSection, "<!-- pp-hermes-install-anchor -->", "")

	// Strip the redundant ## Use with Claude Code section entirely.
	// Its content is now covered by the canonical `## Install` block
	// (which documents `--skill-only` and `--agent` flags), and the MCP
	// `<details>` subsection that lived inside it was unstructured and
	// inconsistent across CLIs.
	body = stripH2Section(body, "## Use with Claude Code")

	// Strip stale alternate-install sections from wherever they are.
	// List both Hermes/OpenClaw naming variants so we can migrate "via"
	// to "for" without a separate code path; strip Claude Desktop so
	// the re-insert below can place it at the canonical position.
	for _, h := range []string{
		"## Install via Hermes",
		"## Install via OpenClaw",
		"## Install for Hermes",
		"## Install for OpenClaw",
		"## Use with Claude Desktop",
	} {
		body = stripH2Section(body, h)
	}
	// Strip the anchor comment with or without trailing newline.
	body = strings.ReplaceAll(body, "<!-- pp-hermes-install-anchor -->\n", "")
	body = strings.ReplaceAll(body, "<!-- pp-hermes-install-anchor -->", "")
	// Collapse any blank-line gaps left by stripping (e.g. two adjacent
	// stripped sections leave `\n\n\n` between surrounding content).
	for strings.Contains(body, "\n\n\n") {
		body = strings.ReplaceAll(body, "\n\n\n", "\n\n")
	}

	// Build the canonical block: anchor + Hermes + OpenClaw + Claude
	// Desktop (when the CLI has one). The Claude Desktop section is
	// passed through verbatim.
	canonical := "<!-- pp-hermes-install-anchor -->\n" + buildReadmeInstallSections(ctx)
	if claudeDesktopSection != "" {
		// Ensure exactly one blank line separates the OpenClaw block
		// (which ends with "\n\n") from the moved Claude Desktop
		// section. extractH2Section preserves the section's own
		// trailing blank line; no extra separator needed.
		canonical += claudeDesktopSection
	}

	installIdx := strings.Index(body, installHeading)
	if installIdx < 0 {
		// Defensive: shouldn't reach here given the early-return above,
		// but keep the guard in case body got transformed unexpectedly.
		return body
	}
	// Find the next H2 after ## Install. That's the section boundary
	// where we insert (right before whatever comes next, typically
	// ## Authentication or ## Quick Start).
	tail := body[installIdx+len(installHeading):]
	nextH2Idx := strings.Index(tail, "\n## ")
	if nextH2Idx < 0 {
		// ## Install is the last section in the README. Append at
		// EOF, ensuring a trailing newline boundary.
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return body + "\n" + canonical
	}
	insertPos := installIdx + len(installHeading) + nextH2Idx + 1
	return body[:insertPos] + canonical + body[insertPos:]
}

// extractH2Section returns the full `## <heading>` section verbatim
// (heading line + body up to the next `## ` heading or EOF). Returns
// "" if the heading is not present. The returned content includes the
// section's trailing blank line so it can be concatenated directly
// without needing a separator.
func extractH2Section(body, heading string) string {
	needle := heading + "\n"
	idx := strings.Index(body, needle)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(needle):]
	nextIdx := strings.Index(rest, "\n## ")
	if nextIdx < 0 {
		// Section runs to EOF.
		return body[idx:]
	}
	return body[idx : idx+len(needle)+nextIdx+1]
}

// stripH2Section removes a `## <heading>` section (heading line + body
// up to the next `## ` heading or EOF) from body. Returns body
// unchanged if the heading is not present. Preserves the blank line
// from the previous section's trailing content; surrounding content
// is otherwise byte-preserved.
func stripH2Section(body, heading string) string {
	needle := heading + "\n"
	idx := strings.Index(body, needle)
	if idx < 0 {
		return body
	}
	rest := body[idx+len(needle):]
	nextIdx := strings.Index(rest, "\n## ")
	var sectionEnd int
	if nextIdx < 0 {
		sectionEnd = len(body)
	} else {
		sectionEnd = idx + len(needle) + nextIdx + 1
	}
	return body[:idx] + body[sectionEnd:]
}

func buildReadmeInstallSections(ctx patchReadmeCtx) string {
	return fmt.Sprintf("## Install for Hermes\n\n"+
		"Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%%LOCALAPPDATA%%\\Programs\\PrintingPress\\bin` on Windows.\n\n"+
		"```bash\n"+
		"npx -y @mvanhorn/printing-press-library install %s --cli-only\n"+
		"```\n\n"+
		"Then install the focused Hermes skill.\n\n"+
		"From the Hermes CLI:\n\n"+
		"```bash\n"+
		"hermes skills install mvanhorn/printing-press-library/cli-skills/pp-%s --force\n"+
		"```\n\n"+
		"Inside a Hermes chat session:\n\n"+
		"```bash\n"+
		"/skills install mvanhorn/printing-press-library/cli-skills/pp-%s --force\n"+
		"```\n\n"+
		"Restart the Hermes session or gateway if the newly installed skill is not visible immediately.\n\n"+
		"## Install for OpenClaw\n\n"+
		"Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%%LOCALAPPDATA%%\\Programs\\PrintingPress\\bin` on Windows):\n\n"+
		"```bash\n"+
		"npx -y @mvanhorn/printing-press-library install %s --agent openclaw\n"+
		"```\n\n"+
		"Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.\n\n",
		ctx.APIName, ctx.APIName, ctx.APIName, ctx.APIName,
	)
}

// ---------------------------------------------------------------------------
// creator + contributors attribution model (issue #900)
//
// Migrates published CLIs from the legacy owner/printer attribution to the
// generator's creator + contributors model (cli-printing-press branch
// tmchow/Nautilus-3). The target shapes below are pinned from that branch's
// templates, not invented:
//
//   - manifest .printing-press.json: add `creator {handle,name}` (+ optional
//     `contributors[]`) right after cli_name, matching the CLIManifest struct
//     field order; legacy owner/printer/printer_name are preserved as the
//     dual-write (creator is derived from them, so they stay consistent).
//   - every generated .go: `// Copyright YYYY <slug>.` → `// Copyright YYYY
//     <creator name> and contributors.` (constant suffix regardless of count).
//   - README byline: `Printed by [@h](…) (Name).` → `Created by [@h](…)
//     (Name).` plus an optional `Contributors:` line.
//   - NOTICE: copyright-holder line, a per-CLI Created by / Contributors block,
//     and the machine-author credit `by Matt Van Horn and Trevin Chow`.
//
// Every function is idempotent in the second-run-zero-diff sense.
// ---------------------------------------------------------------------------

// fileEdit is one pending write: the resolved path plus before/after bytes.
// Collected across every surface so sweepCLI can write them atomically-ish and
// roll back all of a CLI's edits if any single write fails.
type fileEdit struct {
	path   string
	before []byte
	after  []byte
}

// jsonString renders s as a JSON string literal (quoted, escaped), so emitted
// manifest values are valid JSON rather than Go-quoted (the two diverge on
// some Unicode, though display names here are ASCII).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// personFields renders a person's handle/name as indented JSON object fields
// (omitempty, handle before name), without the enclosing braces. Matches
// json.MarshalIndent output for spec.Person at the given field indent.
func personFields(p person, indent string) string {
	var fields []string
	if p.Handle != "" {
		fields = append(fields, indent+jsonString("handle")+": "+jsonString(p.Handle))
	}
	if p.Name != "" {
		fields = append(fields, indent+jsonString("name")+": "+jsonString(p.Name))
	}
	return strings.Join(fields, ",\n") + "\n"
}

// renderCreatorBlock renders the `"creator": { … },` object at the manifest's
// top-level indent (base), with a trailing comma since fields follow it. Nested
// fields sit one indent unit deeper (base+base), matching json.MarshalIndent.
func renderCreatorBlock(p person, base string) string {
	return base + "\"creator\": {\n" + personFields(p, base+base) + base + "},\n"
}

// renderContributorsBlock renders the `"contributors": [ … ],` array at the
// manifest's top-level indent (base), with a trailing comma since legacy fields
// follow it.
func renderContributorsBlock(cs []person, base string) string {
	item := base + base         // array-element brace indent
	field := base + base + base // person-field indent
	var b strings.Builder
	b.WriteString(base + "\"contributors\": [\n")
	for i, c := range cs {
		b.WriteString(item + "{\n")
		b.WriteString(personFields(c, field))
		if i < len(cs)-1 {
			b.WriteString(item + "},\n")
		} else {
			b.WriteString(item + "}\n")
		}
	}
	b.WriteString(base + "],\n")
	return b.String()
}

// manifestIndent returns the leading whitespace of the top-level `cli_name`
// line — the manifest's indent unit. Most manifests use 2 spaces; a few use 4.
// Returns "" when cli_name can't be located at line start.
func manifestIndent(raw string) string {
	re := regexp.MustCompile(`(?m)^([ \t]+)"cli_name":`)
	m := re.FindStringSubmatch(raw)
	if m == nil {
		return ""
	}
	return m[1]
}

// hasTopLevelManifestKey reports whether the manifest already carries the named
// top-level key at the given indent. Nested person keys sit deeper, so the
// `\n<base>"key"` probe never matches a nested handle/name.
func hasTopLevelManifestKey(raw, base, key string) bool {
	return strings.Contains(raw, "\n"+base+jsonString(key)+":")
}

// patchManifest inserts the creator (and, when non-empty, contributors) JSON
// blocks into a raw .printing-press.json, immediately after the cli_name line,
// matching the generator's field order and the manifest's own indent width.
// Idempotent: an existing top-level `creator` key is left untouched;
// contributors are inserted only when the key is absent (so a re-run after a
// manual backfill edit never duplicates).
func patchManifest(raw string, creator person, contributors []person) (string, bool) {
	base := manifestIndent(raw)
	if base == "" {
		return raw, false
	}
	changed := false
	if !hasTopLevelManifestKey(raw, base, "creator") {
		if out, ok := insertAfterCLINameLine(raw, renderCreatorBlock(creator, base)); ok {
			raw = out
			changed = true
		}
	}
	if len(contributors) > 0 && !hasTopLevelManifestKey(raw, base, "contributors") {
		if out, ok := insertAfterCreatorBlock(raw, renderContributorsBlock(contributors, base), base); ok {
			raw = out
			changed = true
		}
	}
	return raw, changed
}

// insertAfterCLINameLine inserts block right after the `"cli_name": "…",` line.
func insertAfterCLINameLine(raw, block string) (string, bool) {
	idx := strings.Index(raw, "\"cli_name\":")
	if idx < 0 {
		return raw, false
	}
	nl := strings.Index(raw[idx:], "\n")
	if nl < 0 {
		return raw, false
	}
	pos := idx + nl + 1 // first byte after the newline ending the cli_name line
	return raw[:pos] + block + raw[pos:], true
}

// insertAfterCreatorBlock inserts block right after the top-level creator
// object's closing `<base>},` line.
func insertAfterCreatorBlock(raw, block, base string) (string, bool) {
	idx := strings.Index(raw, base+"\"creator\": {")
	if idx < 0 {
		return raw, false
	}
	closer := "\n" + base + "},\n"
	rel := strings.Index(raw[idx:], closer)
	if rel < 0 {
		return raw, false
	}
	pos := idx + rel + len(closer)
	return raw[:pos] + block + raw[pos:], true
}

// copyrightHolderLineRe matches a generated copyright header, capturing the
// `// Copyright YYYY ` prefix (1), the holder (2, non-greedy up to the first
// `. Licensed`), and the ` Licensed under …` suffix (3). Anchored per-line so
// `.+?`/`.*` stay within the comment.
var copyrightHolderLineRe = regexp.MustCompile(`(?m)^(// Copyright \d+ )(.+?)\.( Licensed under .*)$`)

// patchCopyrightHeaderContent rewrites the first copyright header in a .go file
// to `<name> and contributors`, preserving the year and the `Licensed under …`
// suffix. Idempotent: a holder already ending in " and contributors" is left
// untouched.
func patchCopyrightHeaderContent(content, name string) (string, bool) {
	loc := copyrightHolderLineRe.FindStringSubmatchIndex(content)
	if loc == nil {
		return content, false
	}
	m := copyrightHolderLineRe.FindStringSubmatch(content)
	if strings.HasSuffix(m[2], " and contributors") {
		return content, false
	}
	replacement := m[1] + name + " and contributors." + m[3]
	return content[:loc[0]] + replacement + content[loc[1]:], true
}

// patchCopyrightHeaders walks cliDir for .go files and returns the pending
// header rewrites. Files without a copyright header (some hand-written helpers)
// are left untouched — the sweep migrates existing headers, it does not add new
// ones.
func patchCopyrightHeaders(cliDir, name string) ([]fileEdit, error) {
	var edits []fileEdit
	err := filepath.WalkDir(cliDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		before, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		after, changed := patchCopyrightHeaderContent(string(before), name)
		if changed {
			edits = append(edits, fileEdit{path: path, before: before, after: []byte(after)})
		}
		return nil
	})
	return edits, err
}

// personLabel renders `<Name> (@<handle>)` (or `(@<handle>)` when nameless),
// matching the NOTICE template's `{{if .Name}}{{.Name}} {{end}}(@{{.Handle}})`.
func personLabel(p person) string {
	if p.Name != "" {
		return p.Name + " (@" + p.Handle + ")"
	}
	return "(@" + p.Handle + ")"
}

// githubLink renders the README byline link `[@h](https://github.com/h) (Name)`
// (parenthetical omitted when nameless), matching readme.md.tmpl.
func githubLink(p person) string {
	s := "[@" + p.Handle + "](https://github.com/" + p.Handle + ")"
	if p.Name != "" {
		s += " (" + p.Name + ")"
	}
	return s
}

// renderByline builds the canonical README byline: a `Created by …` line and,
// when contributors exist, a following `Contributors: …` line (no blank line
// between them — matches the template).
func renderByline(creator person, contributors []person) string {
	line := "Created by " + githubLink(creator) + "."
	if len(contributors) > 0 {
		parts := make([]string, len(contributors))
		for i, c := range contributors {
			parts[i] = githubLink(c)
		}
		line += "\nContributors: " + strings.Join(parts, ", ") + "."
	}
	return line
}

// existingBylineRe matches an existing byline line — legacy `Printed by` or
// already-migrated `Created by` — anchored on the `[@` markdown link so it
// never matches stray prose.
var existingBylineRe = regexp.MustCompile(`(?m)^(Printed|Created) by \[@[^\n]*$`)

// patchReadmeByline replaces an existing byline (and any immediately following
// `Contributors:` line) with the canonical creator + contributors byline; when
// no byline exists it injects one just before `## Install` (where the template
// places it). Idempotent: re-emits identical content on a second run. A
// handle-less creator yields no byline (a link needs a handle) and READMEs
// without a `## Install` anchor are left untouched.
func patchReadmeByline(body string, creator person, contributors []person) string {
	if creator.Handle == "" {
		return body
	}
	canonical := renderByline(creator, contributors)

	if loc := existingBylineRe.FindStringIndex(body); loc != nil {
		end := loc[1]
		if rest := body[end:]; strings.HasPrefix(rest, "\nContributors: ") {
			if nl := strings.Index(rest[1:], "\n"); nl < 0 {
				end = len(body)
			} else {
				end = end + 1 + nl
			}
		}
		return body[:loc[0]] + canonical + body[end:]
	}

	const installHeading = "## Install\n"
	idx := strings.Index(body, installHeading)
	if idx < 0 {
		return body
	}
	return body[:idx] + canonical + "\n\n" + body[idx:]
}

// noticeCopyrightRe matches the NOTICE copyright-holder line (`Copyright YYYY
// <holder>`), capturing the year (1) and holder (2).
var noticeCopyrightRe = regexp.MustCompile(`(?m)^Copyright (\d+) (.+)$`)

// patchNOTICE applies the three NOTICE edits: rewrite the copyright-holder line
// to `<creator name> and contributors`, insert the per-CLI Created by /
// Contributors block before the machine-credit paragraph, and update the
// machine-author credit to `by Matt Van Horn and Trevin Chow`. Idempotent via
// per-edit presence/suffix guards.
func patchNOTICE(content string, creator person, contributors []person) (string, bool) {
	orig := content
	content = patchNoticeCopyrightLine(content, creator.Name)
	content = insertNoticeAttributionBlock(content, creator, contributors)
	content = patchNoticeMachineAuthor(content)
	return content, content != orig
}

// patchNoticeCopyrightLine rewrites the first `Copyright YYYY <holder>` line to
// `<name> and contributors`, preserving the year. No-op when already migrated.
func patchNoticeCopyrightLine(content, name string) string {
	loc := noticeCopyrightRe.FindStringSubmatchIndex(content)
	if loc == nil {
		return content
	}
	m := noticeCopyrightRe.FindStringSubmatch(content)
	if strings.HasSuffix(m[2], " and contributors") {
		return content
	}
	replacement := "Copyright " + m[1] + " " + name + " and contributors"
	return content[:loc[0]] + replacement + content[loc[1]:]
}

// insertNoticeAttributionBlock inserts the per-CLI `Created by …` line and (if
// contributors exist) the `Contributors:` list between the copyright line and
// the machine-credit paragraph. The two blocks are inserted independently so a
// CLI whose NOTICE already has `Created by` from an earlier creator-only sweep
// still gains a `Contributors:` list on a subsequent backfill pass. Idempotent:
// each block is only inserted when its specific marker is absent.
func insertNoticeAttributionBlock(content string, creator person, contributors []person) string {
	content = insertNoticeCreatedByLine(content, creator)
	content = insertNoticeContributorsList(content, contributors)
	return content
}

// insertNoticeCreatedByLine inserts the `Created by` line before the machine-
// credit paragraph when missing. No-op when already present.
func insertNoticeCreatedByLine(content string, creator person) string {
	if strings.Contains(content, "\nCreated by ") {
		return content
	}
	const marker = "\nThis CLI was generated"
	idx := strings.Index(content, marker)
	if idx < 0 {
		return content
	}
	line := "Created by " + personLabel(creator) + ".\n"
	return content[:idx] + "\n" + line + content[idx:]
}

// insertNoticeContributorsList inserts the `Contributors:` list immediately
// after the existing `Created by …` line (preserving its trailing newline)
// when contributors are non-empty and the list is not already present. No-op
// when contributors is empty, when no `Created by` line exists (the
// previous step is responsible for that), or when a `Contributors:` line is
// already present anywhere in the file.
func insertNoticeContributorsList(content string, contributors []person) string {
	if len(contributors) == 0 {
		return content
	}
	if strings.Contains(content, "\nContributors:") {
		return content
	}
	const createdByMarker = "\nCreated by "
	idx := strings.Index(content, createdByMarker)
	if idx < 0 {
		return content
	}
	// Find the newline ending the Created by line.
	lineStart := idx + 1 // skip the leading \n in the marker
	nl := strings.Index(content[lineStart:], "\n")
	if nl < 0 {
		return content
	}
	insertAt := lineStart + nl + 1
	var block strings.Builder
	block.WriteString("Contributors:\n")
	for _, c := range contributors {
		block.WriteString("  - " + personLabel(c) + "\n")
	}
	return content[:insertAt] + block.String() + content[insertAt:]
}

// patchNoticeMachineAuthor updates the Press machine-author credit from
// `by Matt Van Horn.` to `by Matt Van Horn and Trevin Chow.`. No-op when the
// dual credit is already present.
func patchNoticeMachineAuthor(content string) string {
	if strings.Contains(content, "by Matt Van Horn and Trevin Chow") {
		return content
	}
	return strings.Replace(content, "by Matt Van Horn.", "by Matt Van Horn and Trevin Chow.", 1)
}

// ---------------------------------------------------------------------------
// contributor backfill (-backfill-contributors)
//
// Computes each CLI's contributors from `git log` over its library directory,
// excluding the creator and a bot/regen/rename/sweep denylist, and surfaces a
// human-reviewable table before any write. Mirrors the prior authorship-sweep
// scar: never claim others' work, and treat anything we can't confidently map
// to a GitHub handle as "unresolved" for human review rather than guessing.
// ---------------------------------------------------------------------------

// readManifestForDir decodes a CLI directory's .printing-press.json into the
// fields the sweep needs. Used by main() to resolve the creator for the
// contributor table (sweepCLI re-reads it independently for the actual writes).
func readManifestForDir(cliDir string) (manifest, error) {
	data, err := os.ReadFile(filepath.Join(cliDir, ".printing-press.json"))
	if err != nil {
		return manifest{}, err
	}
	var mf manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return manifest{}, err
	}
	return mf, nil
}

// backfillResult holds a CLI's resolved contributors plus the authors we could
// not confidently map to a handle (surfaced for human review, never written).
type backfillResult struct {
	contributors []person
	unresolved   []string // "Name <email>" entries with no resolvable handle
}

// knownHandleByName maps the maintainers who realistically appear as cross-CLI
// contributors to their handles, for the case where a commit's author email is
// not a GitHub noreply address. Deliberately tiny — anyone not here whose email
// isn't a noreply address is reported as unresolved rather than guessed. Keep
// the keys to full, distinctive display names (not generic first names like
// "Benjamin"); use knownHandleByEmail for contributors whose git name is too
// common to identify globally.
var knownHandleByName = map[string]string{
	"Matt Van Horn":  "mvanhorn",
	"Trevin Chow":    "tmchow",
	"Cathryn Lavery": "cathrynlavery",
}

// knownHandleByEmail maps specific commit emails to their GitHub handles, for
// contributors whose git `user.name` is too generic to safely use as a global
// name → handle key (e.g. "Benjamin", "Matt"). Email is the stable identifier
// for those cases. Only add entries here after confirming the email belongs to
// the named GitHub user. Keys are lowercased on lookup.
var knownHandleByEmail = map[string]string{
	"benjamin84@gmail.com": "benjaminn8",
}

// landingOnlyHandles are identities whose git presence across a CLI directory
// reflects repository maintenance — landing PRs, applying cross-cutting fixes,
// running retrofits — rather than authorship of that specific CLI. Crediting
// them as a "contributor" on nearly the whole catalog would dilute the signal
// and misrepresent maintenance as authorship (the prior misattribution scar).
// tmchow is the library's primary maintainer/landing identity: a naive git-log
// backfill credits them on ~154 of 155 CLIs. They are excluded from contributor
// backfill (this never affects creator, which is resolved from the manifest).
// Keyed by lowercased handle.
var landingOnlyHandles = map[string]bool{
	"tmchow": true,
}

// denylistSubjectRe matches commit subjects that are automation/regen/rename/
// sweep noise rather than substantive contribution to a CLI.
var denylistSubjectRe = regexp.MustCompile(`(?i)(chore\(registry\)|chore\(skills\)|fix\(skills\)|fix\(verify-skill\)|chore\(catalog\)|\[skip ci\]|sweep|canonical shape|retrofit|\brename\b|\bmove\b|relocat)`)

// isDenylistedSubject reports whether a commit subject is automation noise.
func isDenylistedSubject(subject string) bool {
	return denylistSubjectRe.MatchString(subject)
}

// isDenylistedAuthor reports whether a commit author is a bot/automation
// identity that should never be credited as a contributor.
func isDenylistedAuthor(name, email string) bool {
	l := strings.ToLower(name + " " + email)
	for _, bad := range []string{"[bot]", "github-actions", "actions-user", "dependabot"} {
		if strings.Contains(l, bad) {
			return true
		}
	}
	return false
}

// handleFromEmail extracts a GitHub handle from a noreply commit email
// (`12345+user@users.noreply.github.com` or `user@users.noreply.github.com`).
// Returns "" for any non-noreply address — we never guess a handle from a
// vanity email local-part.
func handleFromEmail(email string) string {
	const suffix = "@users.noreply.github.com"
	if !strings.HasSuffix(email, suffix) {
		return ""
	}
	local := strings.TrimSuffix(email, suffix)
	if i := strings.Index(local, "+"); i >= 0 {
		local = local[i+1:]
	}
	return local
}

// backfillContributors computes a CLI's contributors from git history. Commits
// are read newest-first (so the most recent contributor leads, approximating
// reprinter-first), denylist-filtered, deduped by handle, and the creator is
// excluded by handle or name. Authors with no resolvable handle are returned as
// unresolved for human review, never as contributors.
func backfillContributors(cliDir string, creator person) backfillResult {
	out, err := exec.Command("git", "log", "--no-merges",
		"--format=%an%x1f%ae%x1f%s", "--", cliDir).Output()
	if err != nil {
		return backfillResult{}
	}

	var res backfillResult
	seenHandle := map[string]bool{}
	seenUnresolved := map[string]bool{}

	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		name, email, subject := parts[0], parts[1], parts[2]
		if isDenylistedAuthor(name, email) || isDenylistedSubject(subject) {
			continue
		}

		handle := handleFromEmail(email)
		if handle == "" {
			handle = knownHandleByEmail[strings.ToLower(email)]
		}
		if handle == "" {
			handle = knownHandleByName[name]
		}
		if handle == "" {
			note := name + " <" + email + ">"
			if !seenUnresolved[note] {
				seenUnresolved[note] = true
				res.unresolved = append(res.unresolved, note)
			}
			continue
		}

		// The creator is credited separately and is never a contributor.
		if strings.EqualFold(handle, creator.Handle) || strings.EqualFold(name, creator.Name) {
			continue
		}

		key := strings.ToLower(handle)
		// Skip maintenance/landing-only identities (see landingOnlyHandles).
		if landingOnlyHandles[key] {
			continue
		}
		if seenHandle[key] {
			continue
		}
		seenHandle[key] = true
		res.contributors = append(res.contributors, person{Handle: handle, Name: name})
	}
	return res
}

// printContributorTable prints a markdown table of the backfilled contributors
// per CLI, plus any unresolved authors, to stdout — the review surface required
// before contributors are written.
func printContributorTable(dirs []string, byDir map[string][]person, unresolvedByDir map[string][]string) {
	fmt.Println("\n=== Contributor backfill — review before these are written ===")
	fmt.Println()
	fmt.Println("| CLI | Contributors (backfilled from git history) |")
	fmt.Println("|-----|---------------------------------------------|")
	any := false
	for _, dir := range dirs {
		cs := byDir[dir]
		if len(cs) == 0 {
			continue
		}
		any = true
		labels := make([]string, len(cs))
		for i, c := range cs {
			labels[i] = personLabel(c)
		}
		fmt.Printf("| %s | %s |\n", filepath.Base(dir), strings.Join(labels, ", "))
	}
	if !any {
		fmt.Println("| (none) | no contributors found beyond the creator |")
	}

	var unresolvedDirs []string
	for dir := range unresolvedByDir {
		unresolvedDirs = append(unresolvedDirs, dir)
	}
	if len(unresolvedDirs) > 0 {
		sort.Strings(unresolvedDirs)
		fmt.Println("\nUnresolved authors (no GitHub handle — NOT written, review manually):")
		for _, dir := range unresolvedDirs {
			fmt.Printf("  %s: %s\n", filepath.Base(dir), strings.Join(unresolvedByDir[dir], "; "))
		}
	}
	fmt.Println()
}
