package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRegistryFile_ExtractsVideoIDs(t *testing.T) {
	t.Parallel()
	content := `# Content Registry

### Endgame Tips LE
- **Video ID:** 2DaH1zqjI2Y
- **Published:** 2025-09-04
- **Status:** Active

### Wildfire Acid Flask
- **Video ID:** mAMqZLl891s
- **Project files:** ` + "`projects/wildfire-acid/`" + `

### Spellblade Is Dead
- **Video ID:** *(not in youtube-videos.txt)*
- **Published:** 2026-03-17
`
	tmp, err := os.CreateTemp("", "registry-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(content)
	tmp.Close()

	entries, err := ParseRegistryFile(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (the third has no real ID), got %d", len(entries))
	}
	if entries[0].VideoID != "2DaH1zqjI2Y" || entries[0].Title != "Endgame Tips LE" {
		t.Errorf("entry 0 = %+v", entries[0])
	}
	if entries[1].VideoID != "mAMqZLl891s" || entries[1].ProjectDir != "projects/wildfire-acid" {
		t.Errorf("entry 1 = %+v", entries[1])
	}
}

func TestExtractFrameworkLines_BasicSections(t *testing.T) {
	t.Parallel()
	script := `# My Script

## Signal
This is the hook line.

## Belief Shift
The audience now believes X.

## CTA
Click subscribe.
`
	tmp, err := os.CreateTemp("", "script-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()

	sig, bs, cta, err := ExtractFrameworkLines(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sig, "hook") {
		t.Errorf("signal = %q", sig)
	}
	if !strings.Contains(bs, "believes") {
		t.Errorf("belief shift = %q", bs)
	}
	if cta != "Click subscribe." {
		t.Errorf("cta = %q", cta)
	}
}

func TestExtractFrameworkLines_MissingSections(t *testing.T) {
	t.Parallel()
	script := `# Random Script

Just prose with no structured sections.
`
	tmp, err := os.CreateTemp("", "script-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	sig, bs, cta, err := ExtractFrameworkLines(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	if sig != "" || bs != "" || cta != "" {
		t.Errorf("expected all empty, got signal=%q bs=%q cta=%q", sig, bs, cta)
	}
}

func TestFindRegistryFile_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := FindRegistryFile(dir); got != "" {
		t.Errorf("expected '' for missing registry, got %q", got)
	}
	// Now create the file and probe
	if err := os.WriteFile(filepath.Join(dir, "content-registry.md"), []byte("# placeholder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindRegistryFile(dir); got == "" {
		t.Errorf("expected non-empty path when file exists")
	}
}

func TestExpandHome_PassthroughWhenNoTilde(t *testing.T) {
	t.Parallel()
	got, err := expandHome("/absolute/path")
	if err != nil || got != "/absolute/path" {
		t.Errorf("expandHome should pass through absolute paths, got %q err=%v", got, err)
	}
}
