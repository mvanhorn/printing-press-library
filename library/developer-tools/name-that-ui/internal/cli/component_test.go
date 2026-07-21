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

func seedComponentDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "components.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	components := []namethatui.Component{
		{ID: "web/combobox", Platform: "web", Slug: "combobox", Name: "Combobox", Tagline: "Choose or enter", AKA: []string{"combo box"}, Fuzzy: []string{"typeahead menu"}, API: []namethatui.API{{Framework: "React", Symbol: "Combobox"}, {Framework: "SwiftUI", Symbol: "Picker"}, {Framework: "HTML", Symbol: "select"}, {Framework: "ARIA", Symbol: "aria-controls"}}, Prompt: "canonical prompt", DebugPrompt: "debug upstream", Description: "A searchable choice control.", Parts: []namethatui.Part{{ID: "input", Name: "Input", API: "TextField"}}, Related: []string{"web/select", "unresolved"}, SourceURL: "https://example.test/combobox"},
		{ID: "web/select", Platform: "web", Slug: "select", Name: "Select", Tagline: "Choose one", API: []namethatui.API{{Framework: "React", Symbol: "Select"}}, Prompt: "select prompt", DebugPrompt: "select debug", Description: "A fixed choice control.", Parts: []namethatui.Part{}, SourceURL: "https://example.test/select"},
		{ID: "ios/search-field", Platform: "ios", Slug: "search-field", Name: "Search Field", AKA: []string{"search"}, SourceURL: "https://example.test/search"},
		{ID: "web/search-bar", Platform: "web", Slug: "search-bar", Name: "Search Bar", AKA: []string{"search"}, SourceURL: "https://example.test/search-bar"},
	}
	for _, c := range components {
		raw, _ := json.Marshal(c)
		if err := db.Upsert("components", c.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("components", "", len(components)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func runComponent(t *testing.T, db string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	base := []string{"--json", "--no-learn", "component", "--db", db}
	root.SetArgs(append(base, args...))
	err := root.Execute()
	var result map[string]any
	if out.Len() > 0 {
		if uerr := json.Unmarshal(out.Bytes(), &result); uerr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), uerr)
		}
	}
	return result, err
}

func TestComponentCommandFamily(t *testing.T) {
	db := seedComponentDB(t)
	cases := []struct {
		name  string
		args  []string
		check func(t *testing.T, got map[string]any)
	}{
		{"list", []string{"list"}, func(t *testing.T, got map[string]any) {
			results, ok := got["results"].([]any)
			if !ok || len(results) != 4 {
				t.Fatalf("results=%#v", got["results"])
			}
		}},
		{"get", []string{"get", "web/combobox"}, func(t *testing.T, got map[string]any) {
			if got["result"].(map[string]any)["id"] != "web/combobox" {
				t.Fatal(got)
			}
		}},
		{"anatomy", []string{"anatomy", "combobox"}, func(t *testing.T, got map[string]any) {
			if len(got["parts"].([]any)) != 1 {
				t.Fatal(got)
			}
		}},
		{"api", []string{"api", "combobox", "--framework", "react"}, func(t *testing.T, got map[string]any) {
			if len(got["api"].([]any)) != 1 {
				t.Fatal(got)
			}
		}},
		{"prompt", []string{"prompt", "combobox", "--framework", "SwiftUI"}, func(t *testing.T, got map[string]any) {
			if got["prompt"] != "canonical prompt" || len(got["api"].([]any)) != 1 {
				t.Fatal(got)
			}
		}},
		{"debug prompt", []string{"debug-prompt", "combobox"}, func(t *testing.T, got map[string]any) {
			if got["debug_prompt"] != "debug upstream" {
				t.Fatal(got)
			}
		}},
		{"related", []string{"related", "combobox"}, func(t *testing.T, got map[string]any) {
			if len(got["related"].([]any)) != 2 {
				t.Fatal(got)
			}
		}},
		{"compare", []string{"compare", "combobox", "select"}, func(t *testing.T, got map[string]any) {
			if got["left"] == nil || got["right"] == nil || got["differences"] == nil {
				t.Fatal(got)
			}
		}},
		{"guidance", []string{"guidance", "combobox", "--framework", "React"}, func(t *testing.T, got map[string]any) {
			if got["source_url"] == "" || got["prompt"] == "" {
				t.Fatal(got)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runComponent(t, db, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, got)
		})
	}
}

func TestComponentAmbiguousMissingAndDryRun(t *testing.T) {
	db := seedComponentDB(t)
	got, err := runComponent(t, db, "get", "search")
	if err != nil {
		t.Fatal(err)
	}
	if got["ambiguous"] != true || len(got["candidates"].([]any)) != 2 {
		t.Fatalf("expected candidates, got %#v", got)
	}
	_, err = runComponent(t, filepath.Join(t.TempDir(), "missing.db"), "list")
	if err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}
	got, err = runComponent(t, filepath.Join(t.TempDir(), "missing.db"), "get", "anything", "--dry-run")
	if err != nil || got["dry_run"] != true || got["sqlite_opened"] != false {
		t.Fatalf("dry run %#v, %v", got, err)
	}
}

func TestComponentReadCommandsUseOneComponentPlaceholder(t *testing.T) {
	cmd := newComponentCmd(&rootFlags{})
	for _, name := range []string{"get", "anatomy", "api", "prompt", "debug-prompt", "related", "guidance"} {
		subcommand, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if subcommand.Use != name+" <component>" {
			t.Fatalf("%s Use = %q", name, subcommand.Use)
		}
	}
}

func TestComponentJSONCollectionsNeverNull(t *testing.T) {
	db := seedComponentDB(t)
	got, err := runComponent(t, db, "api", "ios/search-field")
	if err != nil {
		t.Fatal(err)
	}
	if apis, ok := got["api"].([]any); !ok || apis == nil {
		t.Fatalf("api must be []: %#v", got["api"])
	}
}
