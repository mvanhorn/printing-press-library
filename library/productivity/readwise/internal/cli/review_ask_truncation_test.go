package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewAskTruncation(t *testing.T) {
	for _, row := range []struct {
		name             string
		docs, highlights int
	}{{"small", 10, 10}, {"at_limit", 50, 50}, {"both_truncated", 60, 60}, {"highlights_truncated", 10, 60}, {"docs_truncated", 60, 10}} {
		t.Run(row.name, func(t *testing.T) {
			testenv.Isolate(t)
			path := filepath.Join(t.TempDir(), "data.db")
			db, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, resource := range []string{"documents", "highlights"} {
				n := row.docs
				if resource == "highlights" {
					n = row.highlights
				}
				for i := 0; i < n; i++ {
					raw := json.RawMessage(fmt.Sprintf(`{"id":%d,"title":"retrieval","text":"retrieval","summary":"retrieval"}`, i+1))
					if _, _, err := db.UpsertBatch(resource, []json.RawMessage{raw}); err != nil {
						t.Fatal(err)
					}
				}
			}
			db.Close()
			var view struct {
				Truncated      *bool           `json:"truncated"`
				ReturnedCounts map[string]int  `json:"returned_counts"`
				Blocks         []novelAskBlock `json:"blocks"`
			}
			out := runNovelCommand(t, "ask", "retrieval", "--no-learn", "--db-path", path)
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatal(err)
			}
			wantDocs, wantHighlights := min(row.docs, 50), min(row.highlights, 50)
			wantTruncated := row.docs > 50 || row.highlights > 50
			count := 0
			for _, b := range view.Blocks {
				count += len(b.Items)
			}
			if view.Truncated == nil || *view.Truncated != wantTruncated || view.ReturnedCounts["docs"] != wantDocs || view.ReturnedCounts["highlights"] != wantHighlights || count != wantDocs+wantHighlights {
				t.Fatalf("truncation/counts: %s", out)
			}
			cmd := RootCmd()
			var md strings.Builder
			cmd.SetOut(&md)
			cmd.SetArgs([]string{"ask", "retrieval", "--format", "md", "--no-learn", "--db-path", path})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(md.String(), "Results truncated") != wantTruncated {
				t.Fatalf("md truncation notice: %s", md.String())
			}
		})
	}
}
