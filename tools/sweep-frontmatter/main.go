// Command sweep-frontmatter applies the Hermes/OpenClaw frontmatter
// alignment shape (cli-printing-press
// docs/plans/2026-05-06-002-feat-hermes-openclaw-frontmatter-alignment-plan.md)
// to every per-CLI library entry in this repo:
//
//   - Strips legacy OpenClaw env-var declarations (requires.env, envVars,
//     primaryEnv) from each library/<cat>/<api>/SKILL.md frontmatter.
//   - Adds the Hermes-recognized top-level fields (version, author,
//     license) after the description: line.
//   - Moves the existing `## CLI Installation` section to immediately
//     after the H1 and rewrites it as `## Prerequisites: Install the
//     CLI` with imperative "you must verify ... do not proceed" wording.
//     For CLIs that lack a `## CLI Installation` section entirely, the
//     Prerequisites section is constructed from manifest data instead.
//   - Inserts `## Install via Hermes` and `## Install via OpenClaw`
//     sections into each library/<cat>/<api>/README.md, anchored on the
//     <!-- pp-hermes-install-anchor --> comment when present, or via
//     a fallback chain (Use with Claude Desktop -> Use with Claude
//     Code -> ## Install -> EOF) for legacy READMEs.
//
// Idempotent: running twice produces zero textual diff on the second run.
// Snapshot-restore: if any per-CLI patch fails, all touched files for
// that CLI are restored from in-memory snapshots before moving on. The
// rest of the sweep continues.
//
// One-shot tool. Once every library entry has been swept, the regen
// workflow takes over for ongoing changes (Hermes/OpenClaw shape lives
// in cli-printing-press's skill.md.tmpl and readme.md.tmpl from now on,
// then verbatim-mirrored to cli-skills/ via .github/workflows/generate-skills.yml).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type manifest struct {
	CLIName              string `json:"cli_name"`
	APIName              string `json:"api_name"`
	PrintingPressVersion string `json:"printing_press_version"`
	OwnerName            string `json:"owner_name"`
}

func main() {
	libraryRoot := "library"
	if v := os.Getenv("SWEEP_LIBRARY_ROOT"); v != "" {
		libraryRoot = v
	}

	ownerName := strings.TrimSpace(os.Getenv("SWEEP_OWNER_NAME"))
	if ownerName == "" {
		out, err := exec.Command("git", "config", "user.name").Output()
		if err == nil {
			ownerName = strings.TrimSpace(string(out))
		}
	}
	if ownerName == "" {
		log.Fatalf("could not resolve owner display name: set SWEEP_OWNER_NAME, or `git config user.name`. " +
			"the value lands as `author:` in the published library/<cat>/<api>/SKILL.md frontmatter.")
	}

	cliDirs, err := findCLIDirs(libraryRoot)
	if err != nil {
		log.Fatalf("walking %s: %v", libraryRoot, err)
	}
	if len(cliDirs) == 0 {
		log.Fatalf("no per-CLI directories found under %s", libraryRoot)
	}

	var processed, skipped, errored int
	for _, dir := range cliDirs {
		status, err := sweepCLI(dir, ownerName)
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

type sweepStatus string

const (
	statusUnchanged sweepStatus = "unchanged" // both files already at canonical shape
	statusSkillOnly sweepStatus = "skill-only"
	statusReadmeOnly sweepStatus = "readme-only"
	statusBoth      sweepStatus = "both"
)

// sweepCLI applies the canonical shape to one library/<cat>/<api>/. The
// snapshot-restore guarantees: on any error from patchSkill or patchReadme,
// every file we wrote so far for this CLI is restored from its snapshot
// before the function returns. Unchanged files are not touched.
func sweepCLI(cliDir, ownerName string) (sweepStatus, error) {
	skillPath := filepath.Join(cliDir, "SKILL.md")
	readmePath := filepath.Join(cliDir, "README.md")
	manifestPath := filepath.Join(cliDir, ".printing-press.json")

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
	// field is omitempty and missing in 35 of 49 legacy manifests, so
	// trusting the on-disk location is more reliable.
	category := filepath.Base(filepath.Dir(cliDir))
	if category == "" {
		category = "other"
	}

	pressVersion := mf.PrintingPressVersion
	if pressVersion == "" {
		// Should not happen for any current library entry, but guard
		// anyway — a sweep that emits version: "" silently is worse
		// than one that fails loudly.
		return statusUnchanged, fmt.Errorf("manifest missing printing_press_version")
	}

	// Manifest's owner_name takes priority over operator's git config —
	// this lets a future regen of an individual CLI preserve original
	// attribution without manual touch-up.
	authorName := mf.OwnerName
	if authorName == "" {
		authorName = ownerName
	}

	skillBefore, err := os.ReadFile(skillPath)
	if err != nil {
		return statusUnchanged, fmt.Errorf("read SKILL.md: %w", err)
	}
	readmeBefore, err := os.ReadFile(readmePath)
	if err != nil {
		return statusUnchanged, fmt.Errorf("read README.md: %w", err)
	}

	skillAfter, err := patchSkill(string(skillBefore), patchSkillCtx{
		CLIName:      mf.CLIName,
		APIName:      mf.APIName,
		Category:     category,
		PressVersion: pressVersion,
		AuthorName:   authorName,
	})
	if err != nil {
		return statusUnchanged, fmt.Errorf("patch SKILL.md: %w", err)
	}

	readmeAfter, err := patchReadme(string(readmeBefore), patchReadmeCtx{
		CLIName: mf.CLIName,
		APIName: mf.APIName,
	})
	if err != nil {
		return statusUnchanged, fmt.Errorf("patch README.md: %w", err)
	}

	skillChanged := skillAfter != string(skillBefore)
	readmeChanged := readmeAfter != string(readmeBefore)
	if !skillChanged && !readmeChanged {
		return statusUnchanged, nil
	}

	// Snapshot-restore: track which files we've written so we can roll
	// back on later failures within this CLI's patch set.
	var written []struct{ path string; before []byte }
	defer func() {
		// no-op on success path; the named return below clears this on success
	}()
	rollback := func() {
		for _, w := range written {
			if rerr := os.WriteFile(w.path, w.before, 0o644); rerr != nil {
				fmt.Printf("    WARN restore %s failed: %v\n", w.path, rerr)
			}
		}
	}

	if skillChanged {
		if err := os.WriteFile(skillPath, []byte(skillAfter), 0o644); err != nil {
			return statusUnchanged, fmt.Errorf("write SKILL.md: %w", err)
		}
		written = append(written, struct{ path string; before []byte }{skillPath, skillBefore})
	}
	if readmeChanged {
		if err := os.WriteFile(readmePath, []byte(readmeAfter), 0o644); err != nil {
			rollback()
			return statusUnchanged, fmt.Errorf("write README.md: %w", err)
		}
		written = append(written, struct{ path string; before []byte }{readmePath, readmeBefore})
	}

	switch {
	case skillChanged && readmeChanged:
		return statusBoth, nil
	case skillChanged:
		return statusSkillOnly, nil
	default:
		return statusReadmeOnly, nil
	}
}

type patchSkillCtx struct {
	CLIName      string // e.g. "shopify-pp-cli"
	APIName      string // e.g. "shopify"
	Category     string // e.g. "commerce"
	PressVersion string // e.g. "3.10.0"
	AuthorName   string // display name, e.g. "Trevin Chow"
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
//   - Adds `version`, `author`, `license` top-level fields after the
//     `description:` line if not already present.
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

// ensureFrontmatterTopLevelFields adds `version`, `author`, `license`
// after the `description:` line if those fields aren't already present
// at the top level. Idempotent — if all three are already in the
// frontmatter, the function is a no-op.
func ensureFrontmatterTopLevelFields(fm string, ctx patchSkillCtx) string {
	hasVersion := topLevelFieldRe("version").MatchString(fm)
	hasAuthor := topLevelFieldRe("author").MatchString(fm)
	hasLicense := topLevelFieldRe("license").MatchString(fm)
	if hasVersion && hasAuthor && hasLicense {
		return fm
	}

	lines := strings.Split(fm, "\n")
	var inserted bool
	var out []string
	for _, line := range lines {
		out = append(out, line)
		if inserted {
			continue
		}
		// Insert immediately after the `description:` line. Description
		// values can be very long, but they're always single-line in
		// emitted SKILL.md frontmatter, so finding the prefix is enough.
		if strings.HasPrefix(line, "description:") {
			if !hasVersion {
				out = append(out, fmt.Sprintf("version: %q", ctx.PressVersion))
			}
			if !hasAuthor {
				out = append(out, fmt.Sprintf("author: %q", ctx.AuthorName))
			}
			if !hasLicense {
				out = append(out, `license: "Apache-2.0"`)
			}
			inserted = true
		}
	}
	return strings.Join(out, "\n")
}

// topLevelFieldRe returns a regexp matching `<name>:` at the start of a
// line — i.e. a top-level (non-indented) frontmatter field.
func topLevelFieldRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `:\s`)
}

// patchSkillPrerequisites moves the existing `## CLI Installation`
// section to immediately after the H1 and renames it. For SKILL.md
// files that lack `## CLI Installation` entirely (4 in the live
// library), constructs the section from manifest data.
//
// Idempotent: if `## Prerequisites: Install the CLI` already appears
// anywhere in the body, returns unchanged.
func patchSkillPrerequisites(body string, ctx patchSkillCtx) string {
	if strings.Contains(body, "## Prerequisites: Install the CLI") {
		return body
	}

	prereq := buildPrerequisitesSection(ctx)

	// Try to remove the existing `## CLI Installation` section first.
	body, removed := removeCLIInstallationSection(body)

	// Insert Prerequisites right after the H1 line. If we couldn't find
	// an H1, insert at the very top after the closing frontmatter ---.
	body = insertAfterH1(body, prereq)

	// If we removed the existing CLI Installation section, also update
	// any remaining references (the Direct Use section's "see CLI
	// Installation above" hint is the canonical one, but other prose
	// may reference it too).
	if removed {
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

func buildPrerequisitesSection(ctx patchSkillCtx) string {
	module := fmt.Sprintf(
		"github.com/mvanhorn/printing-press-library/library/%s/%s/cmd/%s",
		ctx.Category, ctx.APIName, ctx.CLIName,
	)
	return fmt.Sprintf(`## Prerequisites: Install the CLI

This skill drives the `+"`%s`"+` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Check Go is installed: `+"`go version`"+` (requires Go 1.23+)
2. Install:
   `+"```bash"+`
   go install %s@latest
   `+"```"+`
3. Verify: `+"`%s --version`"+`
4. Ensure `+"`$GOPATH/bin`"+` (or `+"`$HOME/go/bin`"+`) is on `+"`$PATH`"+`.

If `+"`--version`"+` reports "command not found" after install, the install step did not put the binary on `+"`$PATH`"+`. Do not proceed with skill commands until verification succeeds.

`, ctx.CLIName, module, ctx.CLIName)
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
	CLIName string
	APIName string
}

// patchReadme inserts the Install via Hermes / Install via OpenClaw
// sections into a README.md. Idempotent: skips if `## Install via
// Hermes` is already present.
//
// Insertion-point fallback chain for legacy READMEs without the
// `<!-- pp-hermes-install-anchor -->` HTML comment:
//
//  1. Right before `## Use with Claude Desktop` (most common)
//  2. Right before `## Use with Claude Code`
//  3. Right before `## Install` (older READMEs)
//  4. End of file
//
// "Right before" means: the new sections appear above the matched
// heading, so they show up in the same neighborhood as related
// install-and-setup content.
func patchReadme(body string, ctx patchReadmeCtx) (string, error) {
	if strings.Contains(body, "## Install via Hermes") {
		return body, nil
	}

	insert := buildReadmeInstallSections(ctx)

	// Anchor wins if present (only fresh prints from the post-U3
	// template have it; legacy READMEs do not).
	const anchor = "<!-- pp-hermes-install-anchor -->"
	if idx := strings.Index(body, anchor); idx >= 0 {
		// The anchor in the live template is followed by a newline
		// then the Hermes heading. We insert right after the anchor's
		// newline so subsequent sweeps find their own emitted sections.
		end := idx + len(anchor)
		if end < len(body) && body[end] == '\n' {
			end++
		}
		return body[:end] + insert + body[end:], nil
	}

	// Fallback chain: insert before the first matching heading.
	for _, heading := range []string{
		"\n## Use with Claude Desktop",
		"\n## Use with Claude Code",
		"\n## Install\n",
	} {
		if idx := strings.Index(body, heading); idx >= 0 {
			// Insert anchor + sections + blank line before the heading.
			pre := body[:idx+1] // include the leading \n
			post := body[idx+1:]
			return pre + anchor + "\n" + insert + post, nil
		}
	}

	// No match — append at EOF.
	suffix := body
	if !strings.HasSuffix(suffix, "\n") {
		suffix += "\n"
	}
	return suffix + "\n" + anchor + "\n" + insert, nil
}

func buildReadmeInstallSections(ctx patchReadmeCtx) string {
	return fmt.Sprintf("## Install via Hermes\n\n"+
		"From the Hermes CLI:\n\n"+
		"```bash\n"+
		"hermes skills install mvanhorn/printing-press-library/cli-skills/pp-%s --force\n"+
		"```\n\n"+
		"Inside a Hermes chat session:\n\n"+
		"```bash\n"+
		"/skills install mvanhorn/printing-press-library/cli-skills/pp-%s --force\n"+
		"```\n\n"+
		"## Install via OpenClaw\n\n"+
		"Tell your OpenClaw agent (copy this):\n\n"+
		"```\n"+
		"Install the pp-%s skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-%s. The skill defines how its required CLI can be installed.\n"+
		"```\n\n",
		ctx.APIName, ctx.APIName, ctx.APIName, ctx.APIName,
	)
}
