// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func runProfileFixture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	root.AddCommand(&cobra.Command{Use: "profile-fixture", RunE: func(cmd *cobra.Command, _ []string) error {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"compact": flags.compact, "quiet": flags.quiet})
	}})
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	err := root.Execute()
	return output.String(), err
}

func requireProfileFixture(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runProfileFixture(t, args...)
	if err != nil {
		t.Fatalf("fixture failed: %v; output: %s", err, out)
	}
	return out
}

func TestProfileStoreSelectionsRemainIndependent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	first := filepath.Join(dir, "first", "profiles.json")
	second := filepath.Join(dir, "second", "profiles.json")
	requireProfileFixture(t, "--profile-store", first, "profile", "save", "shared", "--compact")
	requireProfileFixture(t, "--profile-store", second, "profile", "save", "shared", "--quiet")
	for _, tc := range []struct {
		path           string
		compact, quiet bool
	}{{first, true, false}, {second, false, true}} {
		out := requireProfileFixture(t, "--profile-store", tc.path, "--profile", "shared", "profile-fixture")
		var got map[string]bool
		if err := json.Unmarshal([]byte(out), &got); err != nil || got["compact"] != tc.compact || got["quiet"] != tc.quiet {
			t.Fatalf("wrong selected values: %s (%v)", out, err)
		}
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "profile-store") || strings.Contains(string(data), tc.path) {
			t.Fatal("store selector was captured in a profile")
		}
	}
	secondBefore, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	requireProfileFixture(t, "--profile-store", first, "profile", "save", "shared", "--plain")
	requireProfileFixture(t, "--profile-store", first, "profile", "delete", "shared", "--yes")
	secondAfter, err := os.ReadFile(second)
	if err != nil || !bytes.Equal(secondBefore, secondAfter) {
		t.Fatal("other store changed")
	}
	if _, err := runProfileFixture(t, "--profile-store", first, "profile", "show", "shared"); err == nil {
		t.Fatal("deleted profile still visible")
	}
	requireProfileFixture(t, "--profile-store", second, "profile", "use", "shared")
}

func TestProfileStoreDefaultAndConfigRemainCompatible(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	requireProfileFixture(t, "--config", filepath.Join(home, "first.toml"), "profile", "save", "legacy", "--quiet")
	legacy := filepath.Join(home, ".x-twitter-pp-cli", "profiles.json")
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	requireProfileFixture(t, "--config", filepath.Join(home, "second.toml"), "profile", "show", "legacy")
	if p, err := GetProfile("legacy"); err != nil || p == nil {
		t.Fatal("legacy helper changed")
	}
	if got := ListProfileNames(); !reflect.DeepEqual(got, []string{"legacy"}) {
		t.Fatalf("legacy names: %v", got)
	}
	missing := filepath.Join(home, "isolated", "missing.json")
	if out := requireProfileFixture(t, "--profile-store", missing, "profile", "list", "--json"); strings.TrimSpace(out) != "[]" {
		t.Fatalf("default leaked into selected store: %s", out)
	}
	if _, err := runProfileFixture(t, "--profile-store", missing, "--profile", "legacy", "profile-fixture"); err == nil {
		t.Fatal("missing selected store fell back to default")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("read created selected store")
	}
	after, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("legacy store changed")
	}
}

func TestProfileStoreInvalidSelectionNeverFallsBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	requireProfileFixture(t, "profile", "save", "legacy", "--quiet")
	legacy := filepath.Join(home, ".x-twitter-pp-cli", "profiles.json")
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	malformed := filepath.Join(home, "malformed.json")
	if err := os.WriteFile(malformed, []byte("invalid-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []string{"", " ", home, malformed, filepath.Join(malformed, "profiles.json")} {
		for _, operation := range [][]string{{"profile", "list"}, {"profile", "save", "new", "--quiet"}, {"agent-context"}} {
			args := append([]string{"--profile-store", selected}, operation...)
			if _, err := runProfileFixture(t, args...); err == nil {
				t.Fatalf("invalid selection accepted for %v", operation)
			}
		}
	}
	after, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("invalid selection changed default")
	}
}

func TestProfileStoreRejectsUnrelatedJSONWithoutChangingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	for name, original := range map[string]string{
		"null":              `null`,
		"empty-object":      `{}`,
		"unrelated-object":  `{"settings":{"mode":"local"}}`,
		"null-profiles":     `{"profiles":null}`,
		"array-profiles":    `{"profiles":[]}`,
		"additional-fields": `{"profiles":{},"settings":{"mode":"local"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			selected := filepath.Join(dir, name+".json")
			if err := os.WriteFile(selected, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, operation := range [][]string{{"profile", "list"}, {"profile", "save", "new", "--quiet"}, {"agent-context"}} {
				args := append([]string{"--profile-store", selected}, operation...)
				if _, err := runProfileFixture(t, args...); err == nil {
					t.Fatalf("invalid store accepted for %v", operation)
				}
			}
			after, err := os.ReadFile(selected)
			if err != nil || string(after) != original {
				t.Fatal("invalid store was changed")
			}
		})
	}
	if _, err := os.Stat(filepath.Join(home, ".x-twitter-pp-cli", "profiles.json")); !os.IsNotExist(err) {
		t.Fatal("invalid selected store touched the default store")
	}
}

func TestProfileStoreRelativePathAndReservedOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	selected := filepath.Join("profiles", "scoped.json")
	requireProfileFixture(t, "--profile-store", selected, "profile", "save", "scoped", "--quiet")
	if _, err := os.Stat(selected); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".x-twitter-pp-cli", "profiles.json")); !os.IsNotExist(err) {
		t.Fatal("explicit selection touched default")
	}
	var flags rootFlags
	root := newRootCmd(&flags)
	if err := root.PersistentFlags().Set("profile-store", selected); err != nil {
		t.Fatal(err)
	}
	root.PersistentFlags().Lookup("profile-store").Changed = false
	if err := ApplyProfileToFlags(root, &Profile{Values: map[string]string{"profile-store": "other.json"}}); err != nil {
		t.Fatal(err)
	}
	got, err := root.PersistentFlags().GetString("profile-store")
	if err != nil || got != selected {
		t.Fatalf("profile redirected store: %q (%v)", got, err)
	}
	if _, err := runProfileFixture(t, "--profile-store", selected, "profile", "save", "empty"); err == nil {
		t.Fatal("store selector alone should not create a profile")
	}
}

func TestProfileStoreAgentDiscoveryAndPoisonedOverlay(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	selected := filepath.Join(t.TempDir(), "profiles.json")
	requireProfileFixture(t, "profile", "save", "default-only", "--quiet")
	requireProfileFixture(t, "--profile-store", selected, "profile", "save", "selected-only", "--quiet")
	for _, tc := range []struct {
		args  []string
		names []string
	}{
		{[]string{"agent-context"}, []string{"default-only"}},
		{[]string{"--profile-store", selected, "agent-context"}, []string{"selected-only"}},
	} {
		out := requireProfileFixture(t, tc.args...)
		var ctx struct {
			Profiles []string `json:"available_profiles"`
		}
		if err := json.Unmarshal([]byte(out), &ctx); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ctx.Profiles, tc.names) {
			t.Fatalf("discovery crossed stores: %v", ctx.Profiles)
		}
	}
	redirected := filepath.Join(t.TempDir(), "redirected.json")
	legacy := filepath.Join(home, ".x-twitter-pp-cli", "profiles.json")
	poisoned := map[string]any{"profiles": map[string]Profile{"poisoned": {Name: "poisoned", Values: map[string]string{"profile-store": redirected}}}}
	data, err := json.Marshal(poisoned)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o600); err != nil {
		t.Fatal(err)
	}
	requireProfileFixture(t, "--profile", "poisoned", "profile", "save", "safe-copy", "--quiet")
	if _, err := os.Stat(redirected); !os.IsNotExist(err) {
		t.Fatal("stored selector redirected a default-store invocation")
	}
	if profile, err := GetProfile("safe-copy"); err != nil || profile == nil {
		t.Fatal("save left the selected default store")
	}
}

func TestProfileStoreSaveDoesNotFollowPredictableTempSymlink(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	selected := filepath.Join(dir, "selected.json")
	bystander := filepath.Join(dir, "bystander.json")
	requireProfileFixture(t, "--profile-store", bystander, "profile", "save", "untouched", "--quiet")
	before, err := os.ReadFile(bystander)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bystander, selected+".tmp"); err != nil {
		t.Skipf("symlink fixture unsupported: %v", err)
	}
	requireProfileFixture(t, "--profile-store", selected, "profile", "save", "created", "--quiet")
	after, err := os.ReadFile(bystander)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("save overwrote the bystander store")
	}
	info, err := os.Lstat(selected)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("selected store is not a private regular file: %v", info.Mode())
	}
	if target, err := os.Readlink(selected + ".tmp"); err != nil || target != bystander {
		t.Fatal("save touched the pre-existing symlink")
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".profiles-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
	if _, err := runProfileFixture(t, "--profile-store", selected, "profile", "show", "untouched"); err == nil {
		t.Fatal("selected store was retargeted")
	}
	requireProfileFixture(t, "--profile-store", selected, "profile", "show", "created")
}

func TestProfileStoreSaveCleansUpAfterRenameFailure(t *testing.T) {
	dir := t.TempDir()
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{}}, dir); err == nil || !strings.Contains(err.Error(), "replacing profile store") {
		t.Fatalf("rename failure not propagated: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(dir), ".profiles-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files left after rename failure: %v", matches)
	}
}
