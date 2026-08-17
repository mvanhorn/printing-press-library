package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncCommandRequiresArxivQueryScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{
		"sync",
		"--dry-run",
		"--agent",
		"--db", filepath.Join(t.TempDir(), "data.db"),
		"--max-pages", "1",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("sync without --search-query or --id-list succeeded; expected usage error")
	}
	if !strings.Contains(err.Error(), "--search-query or --id-list") {
		t.Fatalf("error = %q, want mention of --search-query or --id-list", err)
	}
}

func TestSyncCommandRejectsBlankIDListScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{
		"sync",
		"--dry-run",
		"--agent",
		"--db", filepath.Join(t.TempDir(), "data.db"),
		"--max-pages", "1",
		"--id-list", ",",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("sync with blank --id-list succeeded; expected usage error")
	}
	if !strings.Contains(err.Error(), "--search-query or --id-list") {
		t.Fatalf("error = %q, want mention of --search-query or --id-list", err)
	}
}

func TestSyncCommandAcceptsSearchQueryScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{
		"sync",
		"--dry-run",
		"--agent",
		"--db", filepath.Join(t.TempDir(), "data.db"),
		"--max-pages", "1",
		"--search-query", "cat:cs.AI",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync with --search-query failed: %v", err)
	}
}

func TestSyncCommandAcceptsIDListScope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{
		"sync",
		"--dry-run",
		"--agent",
		"--db", filepath.Join(t.TempDir(), "data.db"),
		"--max-pages", "1",
		"--id-list", "1706.03762",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync with --id-list failed: %v", err)
	}
}
