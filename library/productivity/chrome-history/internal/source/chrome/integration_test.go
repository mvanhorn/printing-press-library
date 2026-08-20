package chrome

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
)

// seedChromeDB builds an in-memory snapshot with known visits:
//   - broad.test: 30 distinct URLs, 1 visit each (30 visits total) — a
//     breadth-heavy domain whose URLs would be truncated by a per-URL LIMIT.
//   - narrow.test: 2 URLs, 10 visits each (20 visits total) — a few high-count
//     URLs that dominate a per-URL ranking.
//
// Total = 50 visits. A correct domain ranking puts broad.test (30) above
// narrow.test (20); the pre-fix per-URL LIMIT would undercount broad.test.
func seedChromeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	stmts := []string{
		`CREATE TABLE urls(id INTEGER PRIMARY KEY, url TEXT, title TEXT, visit_count INTEGER, typed_count INTEGER, hidden INTEGER DEFAULT 0, last_visit_time INTEGER)`,
		`CREATE TABLE visits(id INTEGER PRIMARY KEY, url INTEGER, visit_time INTEGER, from_visit INTEGER DEFAULT 0, transition INTEGER DEFAULT 0, visit_duration INTEGER DEFAULT 0)`,
		`CREATE TABLE visit_source(id INTEGER PRIMARY KEY, source INTEGER)`,
		`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	now := timeToChromeMicros(time.Now().UTC().Add(-time.Hour))
	urlID, visitID := 0, 0
	addVisit := func(host, path, title string, n int) {
		urlID++
		uid := urlID
		if _, err := db.Exec(`INSERT INTO urls(id, url, title, visit_count, typed_count, hidden, last_visit_time) VALUES (?,?,?,?,1,0,?)`,
			uid, "https://"+host+path, title, n, now); err != nil {
			t.Fatalf("seed url: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO history_fts(url, title, search_terms) VALUES (?,?,'')`, "https://"+host+path, title); err != nil {
			t.Fatalf("seed fts: %v", err)
		}
		for i := 0; i < n; i++ {
			visitID++
			if _, err := db.Exec(`INSERT INTO visits(id, url, visit_time, transition) VALUES (?,?,?,1)`, visitID, uid, now); err != nil {
				t.Fatalf("seed visit: %v", err)
			}
			if _, err := db.Exec(`INSERT INTO visit_source(id, source) VALUES (?,1)`, visitID); err != nil {
				t.Fatalf("seed visit_source: %v", err)
			}
		}
	}
	for i := 1; i <= 30; i++ {
		addVisit("broad.test", "/page"+itoa(i), "broad page", 1)
	}
	addVisit("narrow.test", "/a", "narrow alpha", 10)
	addVisit("narrow.test", "/b", "narrow golang tutorial", 10)
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
	db := seedChromeDB(t)
	src := New()
	since := time.Now().UTC().Add(-24 * time.Hour)

	// Limit 1 is the adversarial case: the old per-URL LIMIT (max(10, 5)) would
	// keep only the 5 highest-count URL rows — both narrow.test URLs plus 3
	// broad.test URLs — undercounting broad.test to 3 and ranking it last.
	stats, err := src.DomainStats(db, source.VisitFilter{Since: since, Limit: 1})
	if err != nil {
		t.Fatalf("DomainStats: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(stats))
	}
	if stats[0].Domain != "broad.test" {
		t.Fatalf("top domain = %q, want broad.test", stats[0].Domain)
	}
	if stats[0].VisitSum != 30 {
		t.Fatalf("broad.test visit_sum = %d, want 30 (URLs were truncated before aggregation)", stats[0].VisitSum)
	}
}

func TestSeededProfileSplitCountsAllDomains(t *testing.T) {
	db := seedChromeDB(t)
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
	db := seedChromeDB(t)
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
