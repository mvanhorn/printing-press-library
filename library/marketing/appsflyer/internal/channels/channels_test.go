package channels

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGroupsCoverHighTrafficSources(t *testing.T) {
	g := Default()
	expectMembers := map[string]string{
		"social":       "facebook_int",
		"programmatic": "googleadwords_int",
		"oem":          "xiaomiglobal_int",
		"rewarded":     "tapjoy_int",
	}
	for group, member := range expectMembers {
		got, err := g.Resolve(group)
		if err != nil {
			t.Fatalf("Default()[%q] resolve error: %v", group, err)
		}
		found := false
		for _, src := range got {
			if src == member {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Default()[%q] missing expected member %q (got %v)", group, member, got)
		}
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	g := Default()
	for _, input := range []string{"social", "SOCIAL", "Social", "  social  "} {
		if _, err := g.Resolve(input); err != nil {
			t.Errorf("Resolve(%q) unexpected error: %v", input, err)
		}
	}
}

func TestResolveUnknownGroupListsAvailable(t *testing.T) {
	g := Default()
	_, err := g.Resolve("not-a-group")
	if err == nil {
		t.Fatal("expected error for unknown group, got nil")
	}
	for _, must := range []string{"social", "programmatic"} {
		if !contains(err.Error(), must) {
			t.Errorf("error message missing %q: %v", must, err)
		}
	}
}

func TestLoadOverlaysUserYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPSFLYER_CONFIG_DIR", dir)
	path := filepath.Join(dir, "channels.yaml")
	yaml := `social:
  - my-custom-source
new-group:
  - source-a
  - source-b
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write override yaml: %v", err)
	}
	g, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got := g["social"]; len(got) != 1 || got[0] != "my-custom-source" {
		t.Errorf("user override for social not applied: %v", got)
	}
	if got := g["new-group"]; len(got) != 2 {
		t.Errorf("user-defined new group not added: %v", got)
	}
	if _, ok := g["programmatic"]; !ok {
		t.Error("default groups should still be present after overlay")
	}
}

func TestNamesSorted(t *testing.T) {
	g := Default()
	names := g.Names()
	if len(names) == 0 {
		t.Fatal("Names returned empty slice")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("Names not sorted: %v", names)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
