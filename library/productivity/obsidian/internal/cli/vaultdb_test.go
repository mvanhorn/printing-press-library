// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestVaultDBPath pins the properties the multi-vault design leans on:
// stability (same path → same file), human readability (basename slug), and
// disambiguation of same-named vaults — critically the nested wrapper vs
// inner folder, which share a basename by construction.
func TestVaultDBPath(t *testing.T) {
	dataDir := "/data"
	wrapper := "/Users/x/Desktop/Marketing Specialist"
	inner := "/Users/x/Desktop/Marketing Specialist/Marketing Specialist"

	a := vaultDBPath(dataDir, wrapper)
	b := vaultDBPath(dataDir, inner)

	if a == b {
		t.Fatalf("wrapper and inner vault derived the SAME db file: %s", a)
	}
	if a != vaultDBPath(dataDir, wrapper) {
		t.Errorf("derivation not stable across calls")
	}
	base := filepath.Base(a)
	if want := "vault-marketing-specialist-"; len(base) < len(want) || base[:len(want)] != want {
		t.Errorf("db filename %q lacks readable slug prefix %q", base, want)
	}
	if filepath.Ext(base) != ".db" {
		t.Errorf("db filename %q lacks .db extension", base)
	}
}

func TestSanitizeVaultSlug(t *testing.T) {
	tests := map[string]string{
		"Marketing Specialist": "marketing-specialist",
		"Agent Specialist":     "agent-specialist",
		"Vault (2026)!":        "vault--2026",
		"---":                  "",
	}
	for in, want := range tests {
		if got := sanitizeVaultSlug(in); got != want {
			t.Errorf("sanitizeVaultSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestVaultPointerRoundTrip covers the sync→read handoff: sync records the
// mirror it just wrote; read commands must resolve exactly that DB — and must
// fall back (ok=false) when the pointer is absent or its DB has been deleted.
func TestVaultPointerRoundTrip(t *testing.T) {
	dataDir := t.TempDir()

	// No pointer yet → not ok.
	if _, _, ok := readCurrentVaultPointer(dataDir); ok {
		t.Fatal("readCurrentVaultPointer reported ok with no pointer file")
	}

	vault := "/Users/x/Desktop/Agent Specialist/Agent Specialist"
	dbPath := filepath.Join(dataDir, "vault-agent-specialist-deadbeef.db")
	if err := os.WriteFile(dbPath, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCurrentVaultPointer(dataDir, vault, dbPath); err != nil {
		t.Fatalf("writeCurrentVaultPointer: %v", err)
	}

	gotVault, gotDB, ok := readCurrentVaultPointer(dataDir)
	if !ok || gotVault != vault || gotDB != dbPath {
		t.Fatalf("round trip = (%q, %q, %v), want (%q, %q, true)", gotVault, gotDB, ok, vault, dbPath)
	}

	// Pointer whose DB vanished → not ok (reader falls back to legacy).
	if err := os.Remove(dbPath); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readCurrentVaultPointer(dataDir); ok {
		t.Error("pointer with missing DB file should not report ok")
	}
}

// TestDetectNestedWrapper builds both layouts on disk: the footgun (wrapper
// containing a same-named inner folder full of markdown) and a normal flat
// vault that must NOT trip the warning.
func TestDetectNestedWrapper(t *testing.T) {
	base := t.TempDir()

	// Footgun layout: <base>/Vault/Vault/concepts/x.md, opened at <base>/Vault.
	wrapper := filepath.Join(base, "Vault")
	innerConcepts := filepath.Join(wrapper, "Vault", "concepts")
	if err := os.MkdirAll(innerConcepts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(innerConcepts, "x.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	inner, isWrapper := detectNestedWrapper(wrapper)
	if !isWrapper {
		t.Fatal("nested wrapper layout not detected")
	}
	if want := filepath.Join(wrapper, "Vault"); inner != want {
		t.Errorf("inner = %q, want %q", inner, want)
	}

	// The inner folder itself is NOT a wrapper (its same-named child doesn't exist).
	if _, isWrapper := detectNestedWrapper(filepath.Join(wrapper, "Vault")); isWrapper {
		t.Error("inner content folder falsely detected as wrapper")
	}

	// Flat vault: <base>/Solo/notes.md → not a wrapper.
	solo := filepath.Join(base, "Solo")
	if err := os.MkdirAll(solo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(solo, "notes.md"), []byte("# n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, isWrapper := detectNestedWrapper(solo); isWrapper {
		t.Error("flat vault falsely detected as wrapper")
	}

	// Same-named child with no markdown anywhere inside → not a wrapper.
	decoy := filepath.Join(base, "Decoy")
	if err := os.MkdirAll(filepath.Join(decoy, "Decoy", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, isWrapper := detectNestedWrapper(decoy); isWrapper {
		t.Error("markdown-less same-named child falsely detected as wrapper")
	}
}

// TestWalkVaultFiles pins the --vault-path walker's contract: vault-relative
// slash paths, dot-directories and node_modules skipped, --folder scoping.
func TestWalkVaultFiles(t *testing.T) {
	vault := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(vault, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("README.md", "# r")
	mk("concepts/a.md", "# a")
	mk("sources/b.md", "# b")
	mk(".obsidian/workspace.json", "{}")
	mk(".git/config", "")
	mk("node_modules/pkg/x.md", "# no")

	got, err := walkVaultFiles(vault, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.md", "concepts/a.md", "sources/b.md"}
	if !reflect.DeepEqual(sorted(got), want) {
		t.Errorf("walkVaultFiles = %v, want %v", sorted(got), want)
	}

	scoped, err := walkVaultFiles(vault, "concepts")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scoped, []string{"concepts/a.md"}) {
		t.Errorf("folder-scoped walk = %v, want [concepts/a.md]", scoped)
	}
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
