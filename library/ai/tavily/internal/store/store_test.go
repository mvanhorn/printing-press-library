package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := tempDB(t)
	if db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestDefaultDBPath(t *testing.T) {
	p := DefaultDBPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".tavily-pp-cli", "tavily.db")
	if p != expected {
		t.Errorf("DefaultDBPath() = %q, want %q", p, expected)
	}
}

func TestInsertAndQuerySearchResults(t *testing.T) {
	db := tempDB(t)
	_, err := db.InsertSearchResult("golang testing", "abc123", `[{"url":"https://go.dev"}]`, "Go is great", 0.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	results, err := db.GetSearchResultsByQuery("golang testing")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Query != "golang testing" {
		t.Errorf("expected query 'golang testing', got %q", results[0].Query)
	}
}

func TestInsertAndQueryMapResults(t *testing.T) {
	db := tempDB(t)
	urls := []string{"https://example.com/a", "https://example.com/b"}
	_, err := db.InsertMapResult("https://example.com", urls, 2)
	if err != nil {
		t.Fatal(err)
	}
	results, err := db.GetMapResultsByBaseURL("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URLCount != 2 {
		t.Errorf("expected url_count 2, got %d", results[0].URLCount)
	}
}

func TestInsertAndSearchResearchReports(t *testing.T) {
	db := tempDB(t)
	_, err := db.InsertResearchReport("best practices for Go testing", "This report covers unit testing, integration testing, and benchmarking in Go.", "auto", "numbered", `[]`, 5)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := db.SearchResearchReports("testing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least 1 match")
	}
}

func TestInsertUsageAndGetHistory(t *testing.T) {
	db := tempDB(t)
	_, err := db.InsertUsageSnapshot("pro", 100, 1000, 50, 20, 10, 5, 15)
	if err != nil {
		t.Fatal(err)
	}
	history, err := db.GetUsageHistory(30)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(history))
	}
	if history[0].TotalUsage != 100 {
		t.Errorf("expected total_usage 100, got %d", history[0].TotalUsage)
	}
}

func TestGetStaleContent(t *testing.T) {
	db := tempDB(t)
	// Insert an extracted page
	_, err := db.InsertExtractedPage("https://example.com/page", "some content", "markdown", "extract", "")
	if err != nil {
		t.Fatal(err)
	}
	// With days=0, everything should be stale (fetched_at < now)
	// Actually with days=0 cutoff is now, so a just-inserted item won't be stale.
	// Use a large days value to ensure nothing is stale:
	items, err := db.GetStaleContent(365)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 stale items (just inserted), got %d", len(items))
	}
}
