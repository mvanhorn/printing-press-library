package thparse

import (
	"os"
	"testing"
)

func TestParseCardListFixtures(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		source string
	}{
		{name: "tech", path: "testdata/th-tech.html", source: "category"},
		{name: "popular", path: "testdata/th-popular.html", source: "popular"},
		{name: "scoreboard", path: "testdata/th-scoreboard.html", source: "scoreboard"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			trends, err := ParseCardList(body, tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(trends) == 0 {
				t.Fatal("expected card trends")
			}
			if trends[0].Slug == "" || trends[0].Title == "" {
				t.Fatalf("missing core fields: %+v", trends[0])
			}
		})
	}
}
