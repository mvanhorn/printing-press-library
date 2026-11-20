package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestR2ExportCheckpointAtomic(t *testing.T) {
	for _, complete := range []bool{false, true} {
		name := "progress"
		if complete {
			name = "complete"
		}
		t.Run(name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			before := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			for _, key := range []string{"readwise-export", "books", "highlights"} {
				if err := s.SaveSyncStateAt(key, "old", 7, before); err != nil {
					t.Fatal(err)
				}
			}
			_, err = s.DB().Exec(`CREATE TRIGGER reject_books BEFORE UPDATE ON sync_state WHEN NEW.resource_type='books' BEGIN SELECT RAISE(ABORT,'fixture'); END`)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SaveReadwiseExportCheckpoint("next", "since", 10, 8, 2, time.Now(), complete); err == nil {
				t.Fatal("expected abort")
			}
			for _, key := range []string{"readwise-export", "books", "highlights"} {
				cursor, at, n, err := s.GetSyncState(key)
				if err != nil || cursor != "old" || n != 7 || !at.Equal(before) {
					t.Fatalf("partial checkpoint %s: %s %v %d %v", key, cursor, at, n, err)
				}
			}
		})
	}
}
