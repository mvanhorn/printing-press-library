package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	expectedPath := filepath.Join(dir, ".notebooklm-pp-cli", "notebooklm.db")
	os.Setenv("HOME", dir)
	defer os.Unsetenv("HOME")

	s, err := Open()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if s.Path() != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, s.Path())
	}
}

func TestUpsertAndSearch(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("HOME", dir)
	defer os.Unsetenv("HOME")

	s, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.UpsertNotebook(NotebookRow{
		ID: "test-nb-1", Title: "Test Notebook", SourceCount: 2,
		UpdatedAt: "2026-01-01T00:00:00Z", SyncedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpsertSource(SourceRow{
		ID: "src-1", NotebookID: "test-nb-1", Title: "Source One", Type: "web_page",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.UpsertContent("src-1", "test-nb-1", "Source One", "This is test content about machine learning algorithms.")
	if err != nil {
		t.Fatal(err)
	}

	results, err := s.Search("machine learning", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SourceID != "src-1" {
		t.Errorf("expected src-1, got %s", results[0].SourceID)
	}

	nbCount, _ := s.NotebookCount()
	if nbCount != 1 {
		t.Errorf("expected 1 notebook, got %d", nbCount)
	}

	srcCount, _ := s.SourceCount()
	if srcCount != 1 {
		t.Errorf("expected 1 source, got %d", srcCount)
	}
}
