package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
)

func TestFilterNotebooksByResources(t *testing.T) {
	batch := []nlm.Notebook{
		{ID: "id-a", Title: "A"},
		{ID: "id-b", Title: "B"},
		{ID: "id-c", Title: "C"},
	}

	t.Run("notebooks resource returns all", func(t *testing.T) {
		got := filterNotebooksByResources(batch, []string{"notebooks"})
		if len(got) != 3 {
			t.Fatalf("got %d notebooks, want 3", len(got))
		}
	})

	t.Run("specific id returns match", func(t *testing.T) {
		got := filterNotebooksByResources(batch, []string{"id-b"})
		if len(got) != 1 || got[0].ID != "id-b" {
			t.Fatalf("got %#v, want single id-b", got)
		}
	})

	t.Run("unknown id returns empty", func(t *testing.T) {
		got := filterNotebooksByResources(batch, []string{"nonexistent-id"})
		if len(got) != 0 {
			t.Fatalf("got %d notebooks, want 0", len(got))
		}
	})

	t.Run("empty batch returns empty", func(t *testing.T) {
		got := filterNotebooksByResources(nil, []string{"notebooks"})
		if len(got) != 0 {
			t.Fatalf("got %d notebooks, want 0", len(got))
		}
	})

	t.Run("empty resources returns batch unchanged", func(t *testing.T) {
		got := filterNotebooksByResources(batch, nil)
		if len(got) != 3 {
			t.Fatalf("got %d notebooks, want 3", len(got))
		}
	})
}
