package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewAskIndependentSources(t *testing.T) {
	for _, source := range []string{"documents", "highlights", "empty", "excluded"} {
		t.Run(source, func(t *testing.T) {
			testenv.Isolate(t)
			path := filepath.Join(t.TempDir(), "data.db")
			db, err := store.OpenWithContext(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			resource := source
			if source == "excluded" {
				resource = "highlights"
			}
			if resource != "empty" {
				raw := json.RawMessage(`{"id":1,"title":"retrieval","text":"retrieval context","summary":"retrieval context"}`)
				if _, _, err := db.UpsertBatch(resource, []json.RawMessage{raw}); err != nil {
					t.Fatal(err)
				}
			}
			db.Close()
			args := []string{"ask", "retrieval", "--no-learn", "--db-path", path}
			if source == "excluded" {
				args = append(args, "--include", "docs")
			}
			var view novelAskView
			out := runNovelCommand(t, args...)
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatal(err)
			}
			if source == "empty" || source == "excluded" {
				cmd := RootCmd()
				var md strings.Builder
				cmd.SetOut(&md)
				cmd.SetArgs(append(args, "--format", "md"))
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(strings.TrimSpace(md.String()), "{") || !strings.Contains(md.String(), "sync") {
					t.Fatalf("expected markdown note: %s", md.String())
				}
				if view.Note == "" || len(view.Blocks) != 0 {
					t.Fatalf("empty view: %s", out)
				}
			} else if len(view.Blocks) == 0 || view.Note != "" || view.ReturnedCounts[map[string]string{"documents": "docs", "highlights": "highlights"}[source]] != 1 {
				t.Fatalf("%s populated source ignored or miscounted: %s", source, out)
			}
		})
	}
}
