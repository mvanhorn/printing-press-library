package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

func TestSearchFTSAndPunctuation(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Finance/Invoice-2019.pdf", "file", "I", "2019-01-01T00:00:00Z", 100),
		fixtureRow("/Photos/beach.jpg", "file", "B", "2019-01-01T00:00:00Z", 100),
	)
	data, err := runRead(t, "search", "invoice", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got searchResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Path != "/Finance/Invoice-2019.pdf" {
		t.Fatalf("entries %+v", got.Entries)
	}
	if _, err := runRead(t, "search", `a-b "c`, "--db", db, "--json"); err != nil {
		t.Fatalf("punctuation search: %v", err)
	}
}
