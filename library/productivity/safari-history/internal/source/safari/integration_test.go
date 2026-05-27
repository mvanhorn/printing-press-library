package safari

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

// seedSafariDB builds an in-memory snapshot with known visits:
//   - broad.test: 30 distinct URLs, 1 visit each (30 visits total).
//   - narrow.test: 2 URLs, 10 visits each (20 visits total).
//
// Total = 50 visits; broad.test (30) should outrank narrow.test (20).
func seedSafariDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE history_items(id INTEGER PRIMARY KEY, url TEXT, domain_expansion TEXT, visit_count INTEGER)`,
		`CREATE TABLE history_visits(id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER, redirect_source INTEGER DEFAULT 0)`,
		`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	now := timeToSafariSeconds(time.Now().UTC().Add(-time.Hour))
	itemID, visitID := 0, 0
	addVisits := func(host, path, title string, n int) {
		itemID++
		iid := itemID
		url := "https://" + host + path
		if _, err := db.Exec(`INSERT INTO history_items(id, url, domain_expansion, visit_count) VALUES (?,?,?,?)`, iid, url, host, n); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO history_fts(url, title, search_terms) VALUES (?,?,'')`, url, title); err != nil {
			t.Fatalf("seed fts: %v", err)
		}
		for i := 0; i < n; i++ {
			visitID++
			if _, err := db.Exec(`INSERT INTO history_visits(id, history_item, visit_time, title, origin) VALUES (?,?,?,?,0)`, visitID, iid, now, title); err != nil {
				t.Fatalf("seed visit: %v", err)
			}
		}
	}
	for i := 1; i <= 30; i++ {
		addVisits("broad.test", "/page"+itoa(i), "broad page", 1)
	}
	addVisits("narrow.test", "/a", "narrow alpha", 10)
	addVisits("narrow.test", "/b", "narrow golang tutorial", 10)
	return db
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func TestSeededDomainRanking(t *testing.T) {
	db := seedSafariDB(t)
	src := New()
	since := time.Now().UTC().Add(-24 * time.Hour)
	stats, err := src.DomainStats(db, source.VisitFilter{Since: since, Limit: 1})
	if err != nil {
		t.Fatalf("DomainStats: %v", err)
	}
	if len(stats) != 1 || stats[0].Domain != "broad.test" {
		t.Fatalf("top domain = %+v, want broad.test", stats)
	}
	if stats[0].VisitSum != 30 {
		t.Fatalf("broad.test visit_sum = %d, want 30", stats[0].VisitSum)
	}
}

func TestSeededProfileSplitCountsAllDomains(t *testing.T) {
	db := seedSafariDB(t)
	src := New()
	since := time.Now().UTC().Add(-24 * time.Hour)
	pd, err := src.ProfileAggregates(db, source.VisitFilter{Since: since, Limit: 5})
	if err != nil {
		t.Fatalf("ProfileAggregates: %v", err)
	}
	if pd.Visits != 50 {
		t.Fatalf("profile visits = %d, want 50", pd.Visits)
	}
}

func TestSeededFTSRankPopulated(t *testing.T) {
	db := seedSafariDB(t)
	src := New()
	since := time.Now().UTC().Add(-24 * time.Hour)
	rows, err := src.FullTextSearch(db, "golang tutorial", source.VisitFilter{Since: since, Limit: 5})
	if err != nil {
		t.Fatalf("FullTextSearch: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one FTS hit")
	}
	if rows[0].URL != "https://narrow.test/b" {
		t.Fatalf("top FTS hit = %q, want https://narrow.test/b", rows[0].URL)
	}
	if rows[0].Rank == 0 {
		t.Fatal("FTS rank is 0 — bm25 relevance not surfaced")
	}
}
