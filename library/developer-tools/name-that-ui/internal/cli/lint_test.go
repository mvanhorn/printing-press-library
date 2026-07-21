// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
)

// TestNovelLintHelpWires smoke-tests that the lint command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelLintHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"lint", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("lint --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "lint"} {
		if !strings.Contains(help, want) {
			t.Fatalf("lint --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelLintFindingsBoundariesAmbiguityAndDryRun(t *testing.T) {
	db := seedComponentDB(t)
	path := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(path, []byte("Use a Combobox, combo box, typeahead menu, Input, and Picker with search.\nDo not selectables.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--json", "--no-learn", "lint", path, "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Findings []lintFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Findings) != 6 || got.Findings[0].MatchedPhrase != "Combobox" || got.Findings[0].Line != 1 {
		t.Fatalf("findings = %#v", got.Findings)
	}
	kinds := map[string]bool{}
	for _, finding := range got.Findings {
		kinds[finding.MatchKind] = true
	}
	for _, kind := range []string{"component", "alias", "fuzzy_phrase", "part", "api"} {
		if !kinds[kind] {
			t.Fatalf("missing %s finding: %#v", kind, got.Findings)
		}
	}
	if !got.Findings[5].Ambiguous || len(got.Findings[5].CanonicalCandidates) != 2 {
		t.Fatalf("search ambiguity = %#v", got.Findings[5])
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	home := t.TempDir()
	stdout, _, err = runRootArgs(t, "--json", "--home", home, "lint", path, "--db", missing, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"file_read": false`) || !strings.Contains(stdout, `"sqlite_opened": false`) {
		t.Fatalf("dry run = %s", stdout)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry run created state under --home: %#v", entries)
	}
}

func TestNovelLintRejectsBinaryOversizeAndMissingMirror(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(binary, []byte{'a', 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "lint", binary, "--db", seedComponentDB(t)); err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("binary error = %v", err)
	}
	large := filepath.Join(dir, "large.md")
	if err := os.WriteFile(large, make([]byte, novelMaxFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "lint", large, "--db", seedComponentDB(t)); err == nil || !strings.Contains(err.Error(), "2 MiB") {
		t.Fatalf("oversize error = %v", err)
	}
	text := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(text, []byte("Combobox"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runRootArgs(t, "--no-learn", "lint", text, "--db", filepath.Join(dir, "missing.db")); err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}
}

func TestLintSuppressesPlainWordAPIsAndKeepsShapedAPIsCaseSensitive(t *testing.T) {
	items := []namethatui.Component{{
		Name:      "Widget",
		SourceURL: "https://example.test/widget",
		API: []namethatui.API{
			{Framework: "Example", Symbol: "name"},
			{Framework: "Example", Symbol: "command"},
			{Framework: "Example", Symbol: "CommandPalette"},
			{Framework: "ARIA", Symbol: "aria-label"},
		},
		Parts: []namethatui.Part{
			{ID: "control", Name: "Control", API: "command"},
			{ID: "options", Name: "Options", API: "aria-controls"},
		},
	}}

	findings := lintFindings([]byte("A name and command are ordinary prose. CommandPalette uses aria-label and aria-controls; commandpalette remains prose."), items)
	if len(findings) != 3 {
		t.Fatalf("findings = %#v, want only API-shaped symbols", findings)
	}
	want := map[string]string{"CommandPalette": "api", "aria-label": "api", "aria-controls": "part_api"}
	for _, finding := range findings {
		if kind, ok := want[finding.MatchedPhrase]; !ok || finding.MatchKind != kind {
			t.Fatalf("unexpected finding = %#v", finding)
		}
		delete(want, finding.MatchedPhrase)
	}
	if len(want) != 0 {
		t.Fatalf("missing API-shaped findings: %#v", want)
	}
}

func TestLintSuppressesProseListButKeepsSwiftUICodeSyntax(t *testing.T) {
	items := []namethatui.Component{{
		Name: "Collection", SourceURL: "https://example.test/list",
		API: []namethatui.API{{Framework: "SwiftUI", Symbol: "List"}},
	}}
	findings := lintFindings([]byte("A List can summarize ordinary prose. Use List { rows } for the actual SwiftUI view."), items)
	if len(findings) != 1 || findings[0].MatchedPhrase != "List" || findings[0].MatchKind != "api" {
		t.Fatalf("findings = %#v, want only code-shaped SwiftUI List", findings)
	}
}

func TestLintSuppressesCursorEditorAndAuthTokenProse(t *testing.T) {
	items := []namethatui.Component{
		{Name: "Editor", SourceURL: "https://example.test/editor", API: []namethatui.API{{Framework: "SwiftUI", Symbol: "Editor"}, {Framework: "Example", Symbol: "Cursor"}}},
		{Name: "Token", SourceURL: "https://example.test/token", API: []namethatui.API{{Framework: "Example", Symbol: "Token"}, {Framework: "Example", Symbol: "Auth"}}},
	}
	prose := []byte("Cursor is the editor used for this repository. This token is used for authentication.\nSupported agents include Codex, Cursor, Gemini CLI, and GitHub Copilot; no account, token, cookies, or browser session is required.\nUse Editor(content) and Cursor { selection } for the UI.\n")
	findings := lintFindings(prose, items)
	if len(findings) != 2 {
		t.Fatalf("findings = %#v, want only the two code-shaped API uses", findings)
	}
	matched := map[string]bool{}
	for _, finding := range findings {
		matched[finding.MatchedPhrase] = true
	}
	for _, forbidden := range []string{"Cursor", "Editor", "Token", "Auth"} {
		if matched[forbidden] && !strings.Contains(string(prose), forbidden+"(") && forbidden != "Cursor" {
			t.Fatalf("overloaded prose was reported: %#v", findings)
		}
	}
	if !matched["Editor"] || !matched["Cursor"] {
		t.Fatalf("real API syntax was lost: %#v", findings)
	}
}
