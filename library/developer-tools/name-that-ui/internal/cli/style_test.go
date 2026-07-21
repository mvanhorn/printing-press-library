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

func seedStyleDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "styles.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	styles := []namethatui.Style{
		{ID: "glassmorphism", Slug: "glassmorphism", Name: "Glassmorphism", SourceURL: "https://example.test/glassmorphism", Signals: []namethatui.Signal{{ID: "frosted", Name: "Frosted translucency", Description: "Translucent frosted panels blur the background."}, {ID: "layers", Name: "Layered depth", Description: "Floating layers suggest spatial depth."}}, Sections: []namethatui.Section{{Heading: "Implementation starting points", Text: "Use translucent backgrounds and backdrop blur.", SourceURL: "https://example.test/glassmorphism#implementation"}, {Heading: "Accessibility cautions", Text: "Maintain text contrast over changing backgrounds.", SourceURL: "https://example.test/glassmorphism#accessibility"}, {Heading: "Visual character", Text: "Frosted translucent panels over a colorful background.", SourceURL: "https://example.test/glassmorphism#character"}}},
		{ID: "minimalism", Slug: "minimalism", Name: "Minimalism", SourceURL: "https://example.test/minimalism", Signals: []namethatui.Signal{{ID: "restraint", Name: "Visual restraint", Description: "Use only essential interface elements."}, {ID: "layers", Name: "Layered depth", Description: "Depth is subtle and restrained."}}, Sections: []namethatui.Section{{Heading: "Code", Text: "Start with a small neutral token set.", SourceURL: "https://example.test/minimalism#code"}, {Heading: "Avoid visual noise", Text: "Avoid decoration that competes with primary tasks.", SourceURL: "https://example.test/minimalism#avoid"}}},
		{ID: "neo-minimalism", Slug: "neo-minimalism", Name: "Neo Minimalism", SourceURL: "https://example.test/neo-minimalism", Signals: []namethatui.Signal{}, Sections: []namethatui.Section{{Heading: "Overview", Text: "A contemporary minimal visual language.", SourceURL: "https://example.test/neo-minimalism"}}},
	}
	for _, style := range styles {
		raw, _ := json.Marshal(style)
		if err := db.Upsert("style_details", style.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("style_details", "", len(styles)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func runStyle(t *testing.T, db string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(append([]string{"--json", "--no-learn", "style", "--db", db}, args...))
	err := root.Execute()
	var result map[string]any
	if out.Len() > 0 {
		if uerr := json.Unmarshal(out.Bytes(), &result); uerr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), uerr)
		}
	}
	return result, err
}

func TestStyleCommandFamily(t *testing.T) {
	db := seedStyleDB(t)
	cases := []struct {
		name  string
		args  []string
		check func(*testing.T, map[string]any)
	}{
		{"identify", []string{"identify", "frosted translucent panels"}, func(t *testing.T, got map[string]any) {
			candidates := got["candidates"].([]any)
			if len(candidates) == 0 || candidates[0].(map[string]any)["style"].(map[string]any)["slug"] != "glassmorphism" || len(candidates[0].(map[string]any)["evidence"].([]any)) == 0 {
				t.Fatalf("identify=%#v", got)
			}
		}},
		{"list", []string{"list", "--limit", "2"}, func(t *testing.T, got map[string]any) {
			results := got["results"].([]any)
			if len(results) != 2 || results[0].(map[string]any)["name"] != "Glassmorphism" {
				t.Fatalf("list=%#v", got)
			}
		}},
		{"get", []string{"get", "glassmorphism"}, func(t *testing.T, got map[string]any) {
			if got["result"].(map[string]any)["source_url"] != "https://example.test/glassmorphism" {
				t.Fatalf("get=%#v", got)
			}
		}},
		{"signals", []string{"signals", "Glassmorphism"}, func(t *testing.T, got map[string]any) {
			if len(got["signals"].([]any)) != 2 || got["source_url"] == "" {
				t.Fatalf("signals=%#v", got)
			}
		}},
		{"compare", []string{"compare", "glassmorphism", "minimalism"}, func(t *testing.T, got map[string]any) {
			if got["left"] == nil || got["right"] == nil || len(got["signal_overlap"].([]any)) != 1 || got["section_heading_differences"] == nil {
				t.Fatalf("compare=%#v", got)
			}
		}},
		{"code", []string{"code", "glassmorphism"}, func(t *testing.T, got map[string]any) {
			sections := got["sections"].([]any)
			if len(sections) != 1 || !strings.Contains(sections[0].(map[string]any)["heading"].(string), "Implementation") {
				t.Fatalf("code=%#v", got)
			}
		}},
		{"cautions", []string{"cautions", "glassmorphism"}, func(t *testing.T, got map[string]any) {
			sections := got["sections"].([]any)
			if len(sections) != 1 || !strings.Contains(sections[0].(map[string]any)["heading"].(string), "Accessibility") {
				t.Fatalf("cautions=%#v", got)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runStyle(t, db, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, got)
		})
	}
}

func TestStyleAmbiguityMissingMirrorDryRunAndCollections(t *testing.T) {
	db := seedStyleDB(t)
	got, err := runStyle(t, db, "get", "minimal")
	if err != nil {
		t.Fatal(err)
	}
	if got["ambiguous"] != true || len(got["candidates"].([]any)) != 2 {
		t.Fatalf("ambiguous result=%#v", got)
	}
	_, err = runStyle(t, filepath.Join(t.TempDir(), "missing.db"), "list")
	if err == nil || !strings.Contains(err.Error(), "sync --resources styles") {
		t.Fatalf("missing mirror error=%v", err)
	}
	got, err = runStyle(t, filepath.Join(t.TempDir(), "missing.db"), "get", "anything", "--dry-run")
	if err != nil || got["dry_run"] != true || got["sqlite_opened"] != false || got["data_source"] != "local" {
		t.Fatalf("dry run=%#v err=%v", got, err)
	}
	got, err = runStyle(t, db, "signals", "neo-minimalism")
	if err != nil {
		t.Fatal(err)
	}
	if signals, ok := got["signals"].([]any); !ok || signals == nil {
		t.Fatalf("signals must be []: %#v", got["signals"])
	}
	got, err = runStyle(t, db, "code", "neo-minimalism")
	if err != nil {
		t.Fatal(err)
	}
	if sections, ok := got["sections"].([]any); !ok || sections == nil || got["reason"] == "" {
		t.Fatalf("code without upstream sections=%#v", got)
	}
}

func TestStyleReadCommandsExposeScannerSafeUseStrings(t *testing.T) {
	cmd := newStyleCmd(&rootFlags{})
	for name, want := range map[string]string{
		"signals":  "signals <style>",
		"code":     "code <style>",
		"cautions": "cautions <style>",
	} {
		subcommand, _, err := cmd.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if subcommand.Use != want {
			t.Fatalf("%s Use = %q, want %q", name, subcommand.Use, want)
		}
	}
}
