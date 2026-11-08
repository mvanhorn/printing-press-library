// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func TestReadSyncFileMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.md")
	cases := []struct {
		name      string
		direction string
		starting  bool
		wantErr   bool
	}{
		{"first pull creates it", "pull", true, false},
		{"first push needs it", "push", true, true},
		{"paired both must not read as empty", "both", false, true},
		{"paired push must not read as empty", "push", false, true},
		{"paired pull must not read as empty", "pull", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readSyncFile(missing, tc.direction, tc.starting)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// Pulled cards must come back as the same number of chunks, whatever they hold.
func TestPulledCardsKeepTheirBoundaries(t *testing.T) {
	cards := []string{"alpha", "beta", "## Title\n\nbody under it", "one\n\n---\n\ntwo", "## Heading"}
	parts := make([]string, len(cards))
	for i, md := range cards {
		parts[i] = milanote.CardBlock(md)
	}
	chunks := milanote.ChunkMarkdown(strings.Join(parts, cardSeparator) + "\n")
	if len(chunks) != len(cards) {
		t.Fatalf("got %d chunks for %d cards: %+v", len(chunks), len(cards), chunks)
	}
	for i, md := range cards {
		if chunks[i].MD != md {
			t.Errorf("card %d came back as %q, want %q", i, chunks[i].MD, md)
		}
	}
}

func TestCardBlockEditReplacesMarkers(t *testing.T) {
	text := "before\n\n---\n\n" + milanote.CardBlock("## T\n\nold") + "\n"
	chunks := milanote.ChunkMarkdown(text)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %+v", chunks)
	}
	repl := milanote.CardBlock("new plain text")
	got := applyLineEdits(text, []lineEdit{{start: chunks[1].Start, end: chunks[1].End, text: &repl}}, nil)
	if strings.Contains(got, "<!-- card") || !strings.Contains(got, "new plain text") {
		t.Fatalf("got %q", got)
	}
}

func TestCardBlockKeepsLiteralMarkers(t *testing.T) {
	for _, md := range []string{
		"<!-- card -->\nhello\n<!-- /card -->",
		"<!-- card -->\nonly an opener",
		"only a closer\n<!-- /card -->",
		"<!-- card -->\n<!-- /card -->\n<!-- card 1 -->\nnested",
	} {
		text := "first\n\n---\n\n" + milanote.CardBlock(md) + "\n\n---\n\nlast\n"
		chunks := milanote.ChunkMarkdown(text)
		if len(chunks) != 3 || chunks[1].MD != md || chunks[2].MD != "last" {
			t.Errorf("%q came back as %+v", md, chunks)
		}
	}
	for _, md := range []string{"plain", "## Heading", "- [ ] one\n- [x] two"} {
		if got := milanote.CardBlock(md); got != md {
			t.Errorf("%q should not be wrapped, got %q", md, got)
		}
	}
}

func TestSettlePending(t *testing.T) {
	board := &milanote.BoardPayload{Elements: map[string]*milanote.Element{
		"landed": {ID: "landed", Location: milanote.CanvasAt("b", 1, 1, 1)},
		"old":    {ID: "old", Location: milanote.CanvasAt("b", 2, 2, 2)},
	}}
	rec := &pairRecord{Items: []pairItem{{ID: "old"}, {ID: "landed", Pending: true}, {ID: "lost", Pending: true}}}
	if !settlePending(rec, board) {
		t.Fatal("should report outstanding cards")
	}
	if len(rec.Items) != 2 || rec.Items[1].ID != "landed" || rec.Items[1].Pending {
		t.Fatalf("items = %+v", rec.Items)
	}
	if settlePending(rec, board) {
		t.Fatal("nothing should be outstanding the second time")
	}
}

func TestBoardFileMode(t *testing.T) {
	dir := t.TempDir()
	if m := boardFileMode(filepath.Join(dir, "new.md")); m != 0o600 {
		t.Fatalf("new file mode = %o", m)
	}
	private := filepath.Join(dir, "private.md")
	if err := os.WriteFile(private, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(private, 0o640); err != nil {
		t.Fatal(err)
	}
	if m := boardFileMode(private); m != 0o640 {
		t.Fatalf("existing mode = %o", m)
	}
}
