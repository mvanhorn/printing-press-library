package linkedin

import (
	"os"
	"strings"
	"testing"
)

func TestLoadSnapshotFixture(t *testing.T) {
	path := t.TempDir() + "/fixture.json"
	if err := os.WriteFile(path, []byte(`{"source":"fixture","section_map":{"headline":"  Senior UXR   Manager  ","about":"Privacy and AI evals"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sections, source, err := LoadSnapshotFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	if source != "fixture" {
		t.Fatalf("source=%s", source)
	}
	if len(sections) != 2 {
		t.Fatalf("sections=%d", len(sections))
	}
	for _, s := range sections {
		if s.Name == "headline" && s.Normalized != "Senior UXR Manager" {
			t.Fatalf("normalized=%q", s.Normalized)
		}
	}
}

func TestLoadSnapshotFixtureRejectsDuplicateSections(t *testing.T) {
	path := t.TempDir() + "/fixture.json"
	fixture := `{"sections":[{"name":"headline","raw_text":"old headline"}],"section_map":{"headline":"new headline"}}`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadSnapshotFixture(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate section") {
		t.Fatalf("expected duplicate section error, got %v", err)
	}
}
