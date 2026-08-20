package thparse

import (
	"os"
	"testing"
)

func TestParseSearchResultsFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/th-results.html")
	if err != nil {
		t.Fatal(err)
	}
	trends, err := ParseSearchResults(body, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if len(trends) == 0 {
		t.Fatal("expected search results")
	}
	if trends[0].Source != "search" {
		t.Fatalf("source=%q", trends[0].Source)
	}
}
