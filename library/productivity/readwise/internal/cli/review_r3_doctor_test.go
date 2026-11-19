package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"testing"
)

func TestR3DoctorHidesContinuations(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(defaultDBPath("readwise-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"books", "readwise-export", "readwise-books-list"} {
		if err := db.SaveSyncState(key, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	report := collectCacheReport(context.Background(), "")
	rows, ok := report["resources"].([]map[string]any)
	if !ok {
		t.Fatalf("report=%v", report)
	}
	if len(rows) != 1 || rows[0]["type"] != "books" {
		t.Fatalf("resources=%v", rows)
	}
}
