package thparse

import (
	"os"
	"testing"
)

func TestParseTrendPageFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/th-trend.html")
	if err != nil {
		t.Fatal(err)
	}
	trend, err := ParseTrendPage(body, "https://www.trendhunter.com/trends/ai-clone")
	if err != nil {
		t.Fatal(err)
	}
	if trend.Slug != "ai-clone" {
		t.Fatalf("slug=%q", trend.Slug)
	}
	if trend.Title == "" {
		t.Fatal("expected title")
	}
	if len(trend.FAQ) == 0 {
		t.Fatal("expected FAQ entries")
	}
	if len(trend.RelatedSlugs) == 0 {
		t.Fatal("expected related slugs")
	}
	if trend.TrendID == "" {
		t.Fatal("expected trend id")
	}
}
