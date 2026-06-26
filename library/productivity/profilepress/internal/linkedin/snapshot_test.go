package linkedin

import (
	"os"
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
