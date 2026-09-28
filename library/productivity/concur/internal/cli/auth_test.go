// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestChromeProfileDirNames_CustomNamedProfile covers the amend-2026-09-28
// finding: a Chrome profile directory renamed away from the "Default"/
// "Profile N" convention (confirmed live with a dedicated automation
// profile directory literally named "ConcurAutomation", shown as "Person 1"
// in Chrome's own profile switcher) must still be discoverable. Before this
// patch, chromeProfileDirNames matched only a bare "Default" or a
// "Profile " prefix, so a custom-named directory was invisible to both
// --profile <name> resolution and auto-detection even though its Cookies
// database held a real, active session.
func TestChromeProfileDirNames_CustomNamedProfile(t *testing.T) {
	dataDir := t.TempDir()

	// A real Chrome installation keeps "Local State" at the user-data-dir
	// root, sibling to each profile's own subdirectory.
	localState := `{
		"profile": {
			"info_cache": {
				"Default": {"name": "Your Chrome"},
				"ConcurAutomation": {"name": "Person 1"}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dataDir, "Local State"), []byte(localState), 0o644); err != nil {
		t.Fatalf("failed to write synthetic Local State: %v", err)
	}

	got := chromeProfileDirNames(dataDir)
	sort.Strings(got)
	want := []string{"ConcurAutomation", "Default"}

	if len(got) != len(want) {
		t.Fatalf("chromeProfileDirNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chromeProfileDirNames()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestChromeProfileDirNames_FallsBackWithoutLocalState covers the case
// where "Local State" cannot be read (missing, unreadable, or malformed) --
// behavior must be unchanged from before this patch: only "Default" and a
// "Profile " prefix are recognized, straight off the actual directory
// listing.
func TestChromeProfileDirNames_FallsBackWithoutLocalState(t *testing.T) {
	dataDir := t.TempDir()

	for _, name := range []string{"Default", "Profile 1", "SomeCustomName", "Guest Profile"} {
		if err := os.Mkdir(filepath.Join(dataDir, name), 0o755); err != nil {
			t.Fatalf("failed to create profile dir %q: %v", name, err)
		}
	}
	// Deliberately no "Local State" file written here.

	got := chromeProfileDirNames(dataDir)
	sort.Strings(got)
	want := []string{"Default", "Profile 1"}

	if len(got) != len(want) {
		t.Fatalf("chromeProfileDirNames() = %v, want %v (fallback heuristic should skip SomeCustomName and Guest Profile)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chromeProfileDirNames()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestChromeProfileDirNames_MalformedLocalStateFallsBack covers a Local
// State file that exists but fails to parse (e.g. truncated by a crash) --
// this must fall back cleanly rather than returning zero profiles outright.
func TestChromeProfileDirNames_MalformedLocalStateFallsBack(t *testing.T) {
	dataDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dataDir, "Local State"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("failed to write malformed Local State: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dataDir, "Default"), 0o755); err != nil {
		t.Fatalf("failed to create Default dir: %v", err)
	}

	got := chromeProfileDirNames(dataDir)
	want := []string{"Default"}

	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("chromeProfileDirNames() = %v, want %v", got, want)
	}
}

// TestResolveProfileByName_CustomNamedProfile covers the end-to-end
// --profile <name> path (both the directory name and, separately, the
// Chrome-displayed name) now resolving a custom-named profile instead of
// failing with `Chrome profile "..." not found`.
func TestResolveProfileByName_CustomNamedProfile(t *testing.T) {
	dataDir := t.TempDir()

	localState := `{
		"profile": {
			"info_cache": {
				"Default": {"name": "Your Chrome"},
				"ConcurAutomation": {"name": "Person 1"}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(dataDir, "Local State"), []byte(localState), 0o644); err != nil {
		t.Fatalf("failed to write synthetic Local State: %v", err)
	}
	profileDir := filepath.Join(dataDir, "ConcurAutomation")
	if err := os.Mkdir(profileDir, 0o755); err != nil {
		t.Fatalf("failed to create ConcurAutomation dir: %v", err)
	}
	prefs := `{"profile": {"name": "Person 1"}}`
	if err := os.WriteFile(filepath.Join(profileDir, "Preferences"), []byte(prefs), 0o644); err != nil {
		t.Fatalf("failed to write synthetic Preferences: %v", err)
	}

	restore := setTestChromeChannelDirs(t, dataDir)
	defer restore()

	t.Run("by directory name", func(t *testing.T) {
		got, err := resolveProfileByName("ConcurAutomation")
		if err != nil {
			t.Fatalf("resolveProfileByName(%q) returned error: %v", "ConcurAutomation", err)
		}
		if got.Dir != "ConcurAutomation" {
			t.Errorf("resolved profile Dir = %q, want %q", got.Dir, "ConcurAutomation")
		}
	})

	t.Run("by display name", func(t *testing.T) {
		got, err := resolveProfileByName("Person 1")
		if err != nil {
			t.Fatalf("resolveProfileByName(%q) returned error: %v", "Person 1", err)
		}
		if got.Dir != "ConcurAutomation" {
			t.Errorf("resolved profile Dir = %q, want %q", got.Dir, "ConcurAutomation")
		}
	})
}

// setTestChromeChannelDirs is a test-only seam: chromeChannelDirs() normally
// probes real OS-specific Chrome install locations, which don't exist in a
// test sandbox. It returns a restore func that must be deferred.
//
// NOTE: this relies on chromeChannelDirsOverride, a package-level test hook
// added alongside this patch (see auth.go) purely so this test file can
// substitute a temp directory without touching real Chrome installations.
func setTestChromeChannelDirs(t *testing.T, dataDir string) func() {
	t.Helper()
	prev := chromeChannelDirsOverride
	chromeChannelDirsOverride = func() ([]chromeChannelDir, error) {
		return []chromeChannelDir{{Channel: "Chrome", DataDir: dataDir}}, nil
	}
	return func() { chromeChannelDirsOverride = prev }
}
