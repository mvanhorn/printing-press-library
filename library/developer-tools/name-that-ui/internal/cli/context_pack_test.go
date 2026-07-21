// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

// TestNovelContextPackHelpWires smoke-tests that the context-pack command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelContextPackHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"context-pack", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("context-pack --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "context-pack"} {
		if !strings.Contains(help, want) {
			t.Fatalf("context-pack --help missing %q in output:\n%s", want, help)
		}
	}
}

func seedContextPackDB(t *testing.T) string {
	t.Helper()
	path := seedComponentDB(t)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	style := namethatui.Style{
		ID: "glassmorphism", Slug: "glassmorphism", Name: "Glassmorphism", SourceURL: "https://example.test/glassmorphism",
		Signals: []namethatui.Signal{{ID: "frosted", Name: "Frosted translucency", Description: "Translucent panels."}},
		Sections: []namethatui.Section{
			{Heading: "Implementation starting points", Text: "Use source-backed starting points.", SourceURL: "https://example.test/glassmorphism#implementation"},
			{Heading: "Accessibility cautions", Text: "Maintain contrast.", SourceURL: "https://example.test/glassmorphism#accessibility"},
		},
	}
	raw, _ := json.Marshal(style)
	if err := db.Upsert("style_details", style.ID, raw); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("style_details", "", 1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func runContextPack(t *testing.T, db string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"--json", "--no-learn", "context-pack", "--db", db}, args...))
	err := root.Execute()
	result := map[string]any{}
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), decodeErr)
		}
	}
	return result, err
}

func TestContextPackAssemblesFilteredSourceBackedPacket(t *testing.T) {
	got, err := runContextPack(t, seedContextPackDB(t), "--component", "combobox", "--style", "glassmorphism", "--framework", "React")
	if err != nil {
		t.Fatal(err)
	}
	if got["found"] != true || got["style_found"] != true {
		t.Fatalf("found state = %#v", got)
	}
	component := got["component"].(map[string]any)
	if component["id"] != "web/combobox" || component["source_url"] != "https://example.test/combobox" {
		t.Fatalf("component = %#v", component)
	}
	if apis := got["apis"].([]any); len(apis) != 1 || apis[0].(map[string]any)["framework"] != "React" {
		t.Fatalf("framework-filtered APIs = %#v", got["apis"])
	}
	if got["prompt"] != "canonical prompt" || got["debug_prompt"] != "debug upstream" || len(got["related"].([]any)) != 2 {
		t.Fatalf("upstream component fields = %#v", got)
	}
	if len(got["style_signals"].([]any)) != 1 || len(got["code_sections"].([]any)) != 1 || len(got["cautions"].([]any)) != 1 {
		t.Fatalf("style fields = %#v", got)
	}
	urls := got["source_urls"].([]any)
	if len(urls) != 4 || !containsString(urls, "https://example.test/glassmorphism#accessibility") {
		t.Fatalf("source urls = %#v", urls)
	}
	if provenance := got["provenance"].(map[string]any); provenance["data_source"] != "local" || provenance["component"] == nil || provenance["style"] == nil {
		t.Fatalf("provenance = %#v", provenance)
	}
}

func TestContextPackFrameworkWebIncludesHTMLAndARIA(t *testing.T) {
	db := seedContextPackDB(t)
	got, err := runContextPack(t, db, "--component", "combobox", "--framework", "WEB")
	if err != nil {
		t.Fatal(err)
	}
	apis := got["apis"].([]any)
	if len(apis) != 2 {
		t.Fatalf("web APIs = %#v, want HTML and ARIA only", apis)
	}
	frameworks := map[string]bool{}
	for _, api := range apis {
		frameworks[api.(map[string]any)["framework"].(string)] = true
	}
	if !frameworks["HTML"] || !frameworks["ARIA"] || frameworks["React"] || frameworks["SwiftUI"] {
		t.Fatalf("web frameworks = %#v, want HTML and ARIA only", frameworks)
	}
}

func TestContextPackOptionalAndMissingStyleRemainExplicit(t *testing.T) {
	db := seedContextPackDB(t)
	got, err := runContextPack(t, db, "--component", "combobox")
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := got["style_found"]; exists || len(got["style_signals"].([]any)) != 0 || len(got["code_sections"].([]any)) != 0 || len(got["cautions"].([]any)) != 0 {
		t.Fatalf("optional style packet = %#v", got)
	}
	got, err = runContextPack(t, db, "--component", "combobox", "--style", "missing-style")
	if err != nil {
		t.Fatal(err)
	}
	if got["style_found"] != false || got["style_ambiguous"] != false {
		t.Fatalf("missing style = %#v", got)
	}
	if candidates, ok := got["style_candidates"].([]any); !ok || candidates == nil || len(candidates) != 0 {
		t.Fatalf("style candidates must be []: %#v", got["style_candidates"])
	}
}

func TestContextPackAmbiguityDryRunAndMissingMirror(t *testing.T) {
	db := seedContextPackDB(t)
	got, err := runContextPack(t, db, "--component", "search")
	if err != nil {
		t.Fatal(err)
	}
	if got["found"] != false || got["ambiguous"] != true || len(got["candidates"].([]any)) != 2 {
		t.Fatalf("component ambiguity = %#v", got)
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	_, err = runContextPack(t, missing, "--component", "combobox")
	if err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}
	got, err = runContextPack(t, missing, "--component", "combobox", "--dry-run")
	if err != nil || got["dry_run"] != true || got["sqlite_opened"] != false || got["data_source"] != "local" {
		t.Fatalf("dry run = %#v, %v", got, err)
	}
	got, err = runContextPack(t, missing, "--dry-run")
	if err != nil || got["dry_run"] != true || got["component"] != "" || got["style"] != "" || got["framework"] != "" || got["sqlite_opened"] != false {
		t.Fatalf("empty dry run = %#v, %v", got, err)
	}
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
