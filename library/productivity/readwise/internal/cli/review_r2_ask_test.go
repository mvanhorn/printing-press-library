package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"testing"
)

func TestR2AskChildrenExcludedBeforeLimit(t *testing.T) {
	testenv.Isolate(t)
	db, path := e3DB(t)
	for i := 1; i <= 80; i++ {
		parent := "parent"
		if i > 70 {
			parent = ""
		}
		raw := json.RawMessage(fmt.Sprintf(`{"id":"%03d","title":"retrieval","summary":"retrieval","parent_id":%q}`, i, parent))
		if _, _, err := db.UpsertBatch("documents", []json.RawMessage{raw}); err != nil {
			t.Fatal(err)
		}
	}
	out := runNovelCommand(t, "ask", "retrieval", "--no-learn", "--db-path", path)
	var v struct {
		Truncated bool           `json:"truncated"`
		Counts    map[string]int `json:"returned_counts"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatal(err)
	}
	if v.Truncated || v.Counts["docs"] != 10 {
		t.Fatalf("top-level counts/truncation: %s", out)
	}
}
