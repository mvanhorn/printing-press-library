package store

import (
	"path/filepath"
	"testing"
)

func TestSearchUFOFilesReleaseFilterAppliesBeforeLimit(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.EnsureUFOSchema(); err != nil {
		t.Fatalf("EnsureUFOSchema: %v", err)
	}

	files := []UFOFile{
		{ID: "rel1-a", Title: "radar alpha", Type: "PDF", Agency: "AARO", ReleaseBatch: 1},
		{ID: "rel1-b", Title: "radar beta", Type: "PDF", Agency: "AARO", ReleaseBatch: 1},
		{ID: "rel2-a", Title: "radar gamma", Type: "PDF", Agency: "AARO", ReleaseBatch: 2},
		{ID: "rel2-b", Title: "radar delta", Type: "PDF", Agency: "AARO", ReleaseBatch: 2},
	}
	if stored, err := s.UpsertUFOFileBatch(files); err != nil || stored != len(files) {
		t.Fatalf("UpsertUFOFileBatch stored=%d err=%v, want %d nil", stored, err, len(files))
	}
	if err := s.RebuildFTS(); err != nil {
		t.Fatalf("RebuildFTS: %v", err)
	}

	got, err := s.SearchUFOFiles("radar", 2, 2)
	if err != nil {
		t.Fatalf("SearchUFOFiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2; results = %#v", len(got), got)
	}
	for _, f := range got {
		if f.ReleaseBatch != 2 {
			t.Fatalf("result %s release_batch = %d, want 2", f.ID, f.ReleaseBatch)
		}
	}
}

func TestGetTimelineReleaseFilterAppliesInSQL(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.EnsureUFOSchema(); err != nil {
		t.Fatalf("EnsureUFOSchema: %v", err)
	}

	files := []UFOFile{
		{ID: "rel1-early", Title: "release one early", Type: "PDF", Agency: "AARO", ParsedDate: "1944-01-01", ReleaseBatch: 1},
		{ID: "rel2-mid", Title: "release two mid", Type: "PDF", Agency: "AARO", ParsedDate: "1945-01-01", ReleaseBatch: 2},
		{ID: "rel1-late", Title: "release one late", Type: "PDF", Agency: "AARO", ParsedDate: "1946-01-01", ReleaseBatch: 1},
		{ID: "rel2-late", Title: "release two late", Type: "PDF", Agency: "AARO", ParsedDate: "1947-01-01", ReleaseBatch: 2},
	}
	if stored, err := s.UpsertUFOFileBatch(files); err != nil || stored != len(files) {
		t.Fatalf("UpsertUFOFileBatch stored=%d err=%v, want %d nil", stored, err, len(files))
	}

	got, err := s.GetTimeline("", "", 2)
	if err != nil {
		t.Fatalf("GetTimeline: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2; results = %#v", len(got), got)
	}
	for _, f := range got {
		if f.ReleaseBatch != 2 {
			t.Fatalf("result %s release_batch = %d, want 2", f.ID, f.ReleaseBatch)
		}
	}
}
