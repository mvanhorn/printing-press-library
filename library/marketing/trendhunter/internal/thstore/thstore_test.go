package thstore

import (
	"context"
	"database/sql"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/thparse"
	_ "modernc.org/sqlite"
)

func TestEnsureSchemaUpsertSearch(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	trend := thparse.Trend{
		Slug:        "smart-oven",
		Title:       "Smart Oven Assistants",
		Description: "Connected appliances use AI to cook dinner.",
		Keywords:    []string{"smart home", "appliances", "ai"},
		Category:    "tech",
		Author:      "AICE Agent",
		Source:      "rss",
	}
	if err := UpsertTrend(ctx, db, trend); err != nil {
		t.Fatal(err)
	}
	got, ok, err := GetTrend(ctx, db, "smart-oven")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Title != trend.Title {
		t.Fatalf("got trend=%+v ok=%v", got, ok)
	}
	results, err := SearchTrends(ctx, db, "appliances", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Slug != "smart-oven" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}
