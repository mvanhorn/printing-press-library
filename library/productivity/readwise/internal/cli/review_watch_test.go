package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"strings"
	"testing"
)

func TestReviewWatchPrintsConsumedItems(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			testenv.Isolate(t)
			db, path := e3DB(t)
			for _, id := range []string{"document-long-id-1", "document-long-id-2"} {
				raw, _ := json.Marshal(map[string]string{"id": id, "title": "Reading title", "url": "https://example.test/item"})
				if _, _, err := db.UpsertBatch("documents", []json.RawMessage{raw}); err != nil {
					t.Fatal(err)
				}
			}
			id, err := db.AddWatch(context.Background(), "reading", "docs", "{}")
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"watch", "run", "--db", path, "--no-learn"}
			if format == "json" {
				args = append(args, "--json")
			}
			out, err := e3Run(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"document-long-id-1", "document-long-id-2", "Reading title", "https://example.test/item"} {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q: %s", want, out)
				}
			}
			if format == "json" {
				var v struct {
					Watches []struct {
						Items []map[string]string `json:"items"`
					} `json:"watches"`
				}
				if err := json.Unmarshal([]byte(out), &v); err != nil || len(v.Watches) != 1 || len(v.Watches[0].Items) != 2 {
					t.Fatalf("items: %s %v", out, err)
				}
			}
			seen, err := db.SeenItemIDs(context.Background(), id, "docs")
			if err != nil || len(seen) != 2 {
				t.Fatalf("seen=%v err=%v", seen, err)
			}
			out, err = e3Run(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "document-long-id") {
				t.Fatalf("consumed twice: %s", out)
			}
		})
	}
}
