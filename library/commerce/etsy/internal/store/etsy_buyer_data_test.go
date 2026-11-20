package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const etsyReviewFixture = `{"id":"1000000001","transaction_id":1000000001,"listing_id":1000000002,"shop_id":9000001,"parent_id":"1000000002","rating":5,"review":"ceramic mug","buyer_user_id":1000000003,"image_url_fullxfull":"https://example.invalid/buyerphoto"}`
const etsyReviewCleanFixture = `{"id":"1000000001","transaction_id":1000000001,"listing_id":1000000002,"shop_id":9000001,"parent_id":"1000000002","rating":5,"review":"ceramic mug"}`

func etsyBuyerTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func etsyAssertReviewClean(t *testing.T, data string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"buyer_user_id", "image_url_fullxfull"} {
		if _, ok := obj[key]; ok {
			t.Fatalf("retained %s: %s", key, data)
		}
	}
	if string(obj["review"]) != `"ceramic mug"` {
		t.Fatalf("lost review: %s", data)
	}
}

func TestEtsyReviewStripWritePaths(t *testing.T) {
	for _, typ := range []string{"reviews", "listings_reviews"} {
		for _, path := range []string{"generic", "typed", "batch"} {
			// UpsertReviews always writes the reviews partition; the dependent
			// listings_reviews partition has no typed writer or typed table.
			if path == "typed" && typ != "reviews" {
				continue
			}
			t.Run(typ+"/"+path, func(t *testing.T) {
				s := etsyBuyerTestStore(t)
				write := func(raw string) {
					t.Helper()
					var err error
					switch path {
					case "generic":
						err = s.Upsert(typ, "1000000001", json.RawMessage(raw))
					case "typed":
						err = s.UpsertReviews(json.RawMessage(raw))
					case "batch":
						var n, e, f int
						n, e, f, err = s.UpsertBatchDetailed(typ, []json.RawMessage{json.RawMessage(raw)})
						if n != 1 || e != 0 || f != 0 {
							t.Fatalf("batch: %d %d %d", n, e, f)
						}
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				assert := func() {
					t.Helper()
					var data string
					if err := s.db.QueryRow(`SELECT data FROM resources WHERE resource_type=?`, typ).Scan(&data); err != nil {
						t.Fatal(err)
					}
					etsyAssertReviewClean(t, data)
					if typ == "reviews" && path != "generic" {
						var buyer sql.NullInt64
						var image sql.NullString
						if err := s.db.QueryRow(`SELECT data,buyer_user_id,image_url_fullxfull FROM reviews`).Scan(&data, &buyer, &image); err != nil {
							t.Fatal(err)
						}
						etsyAssertReviewClean(t, data)
						if buyer.Valid || image.Valid {
							t.Fatalf("typed buyer fields: %v %v", buyer, image)
						}
					}
				}
				write(etsyReviewFixture)
				assert()
				// Simulate legacy richer blobs AFTER the migration has completed.
				if _, err := s.db.Exec(`UPDATE resources SET data=? WHERE resource_type=?`, etsyReviewFixture, typ); err != nil {
					t.Fatal(err)
				}
				if typ == "reviews" && path != "generic" {
					if _, err := s.db.Exec(`UPDATE reviews SET data=?,buyer_user_id=1000000003,image_url_fullxfull='https://example.invalid/buyerphoto'`, etsyReviewFixture); err != nil {
						t.Fatal(err)
					}
				}
				write(etsyReviewCleanFixture)
				assert()
			})
		}
	}
	// Non-review buyer fields remain unchanged in this narrowly scoped batch.
	if got := etsyStripReviewBuyerFields("transactions", json.RawMessage(etsyReviewFixture)); string(got) != etsyReviewFixture {
		t.Fatal("changed transactions")
	}
}

func TestEtsyReviewFTSExcludesBuyerFields(t *testing.T) {
	// Test the pre-strip extractor directly: a stripped-write test would be
	// vacuous and could not justify leaving the legacy FTS index untouched.
	if shouldIndexSearchString("buyer_user_id", "1000000003") || shouldIndexSearchString("image_url_fullxfull", "https://example.invalid/buyerphoto") {
		t.Fatal("buyer fields indexable")
	}
	content := searchableResourceContent(json.RawMessage(etsyReviewFixture))
	if content != "ceramic mug" {
		t.Fatalf("legacy FTS content: %q", content)
	}
}

func TestEtsyReviewScrubMigration(t *testing.T) {
	s := etsyBuyerTestStore(t)
	if _, err := s.db.Exec(`DELETE FROM etsy_meta WHERE key=?`, etsyReviewScrubMarker); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"reviews", "listings_reviews", "transactions"} {
		for _, id := range []string{"1000000001", "pp:params:synthetic"} {
			if _, err := s.db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES(?,?,?)`, typ, id, etsyReviewFixture); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES(?,'malformed','{')`, typ); err != nil {
			t.Fatal(err)
		}
	}
	for i, data := range []string{etsyReviewFixture, etsyReviewCleanFixture, `{`, `{"buyer_user_id":null,"image_url_fullxfull":null,"review":"ceramic mug"}`} {
		if _, err := s.db.Exec(`INSERT INTO reviews(id,data,buyer_user_id,image_url_fullxfull) VALUES(?,?,1000000003,'https://example.invalid/buyerphoto')`, []string{"1000000001", "pp:params:synthetic", "malformed", "null-fields"}[i], data); err != nil {
			t.Fatal(err)
		}
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	path := s.Path()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	var markers int
	if err := ro.db.QueryRow(`SELECT count(*) FROM etsy_meta WHERE key=?`, etsyReviewScrubMarker).Scan(&markers); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := ro.db.QueryRow(`SELECT data FROM resources WHERE resource_type='reviews' AND id='1000000001'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if markers != 0 || !strings.Contains(raw, "buyer_user_id") {
		t.Fatal("read-only open scrubbed data")
	}
	ro.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.Query(`SELECT data FROM resources WHERE resource_type IN ('reviews','listings_reviews') AND id!='malformed' UNION ALL SELECT data FROM reviews WHERE data!='{'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		etsyAssertReviewClean(t, raw)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, q := range []string{`SELECT count(*) FROM reviews WHERE buyer_user_id IS NOT NULL OR image_url_fullxfull IS NOT NULL`, `SELECT count(*) FROM resources WHERE resource_type IN ('reviews','listings_reviews') AND id='malformed' AND data!='{'`} {
		var n int
		if err := s.db.QueryRow(q).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d %v", q, n, err)
		}
	}
	if err := s.db.QueryRow(`SELECT data FROM resources WHERE resource_type='transactions' AND id='1000000001'`).Scan(&raw); err != nil || raw != etsyReviewFixture {
		t.Fatalf("transaction changed: %s %v", raw, err)
	}
	var after int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&after); err != nil || after != version {
		t.Fatalf("version: %d %d %v", version, after, err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM etsy_meta WHERE key=? AND value='done'`, etsyReviewScrubMarker).Scan(&markers); err != nil || markers != 1 {
		t.Fatalf("marker: %d %v", markers, err)
	}
	// Count writes with triggers so idempotence checks actual UPDATEs too.
	for _, q := range []string{`CREATE TABLE scrub_updates(n INTEGER)`, `CREATE TRIGGER scrub_resources AFTER UPDATE ON resources BEGIN INSERT INTO scrub_updates VALUES(1); END`, `CREATE TRIGGER scrub_reviews AFTER UPDATE ON reviews BEGIN INSERT INTO scrub_updates VALUES(1); END`} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.etsyScrubReviewBuyerFields(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM scrub_updates`).Scan(&after); err != nil || after != 0 {
		t.Fatalf("second scrub wrote: %d %v", after, err)
	}
}

func TestEtsyReviewScrubAtomicRetry(t *testing.T) {
	s := etsyBuyerTestStore(t)
	for _, q := range []string{
		`DELETE FROM etsy_meta`,
		`CREATE TRIGGER scrub_fail BEFORE UPDATE ON reviews BEGIN SELECT RAISE(ABORT,'synthetic scrub failure'); END`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('reviews','1000000001',?)`, etsyReviewFixture); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO reviews(id,data,buyer_user_id) VALUES('1000000001',?,1000000003)`, etsyReviewFixture); err != nil {
		t.Fatal(err)
	}
	if err := s.etsyScrubReviewBuyerFields(context.Background()); err == nil || !strings.Contains(err.Error(), "synthetic scrub failure") {
		t.Fatalf("expected injected failure: %v", err)
	}
	var raw string
	if err := s.db.QueryRow(`SELECT data FROM resources WHERE resource_type='reviews'`).Scan(&raw); err != nil || raw != etsyReviewFixture {
		t.Fatalf("partial scrub committed: %s %v", raw, err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM etsy_meta`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("marker committed: %d %v", n, err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER scrub_fail`); err != nil {
		t.Fatal(err)
	}
	// A trigger on the update's own connection verifies secure_delete is ON
	// where the scrub executes, rather than on an unrelated pooled handle.
	if _, err := s.db.Exec(`CREATE TRIGGER scrub_secure BEFORE UPDATE ON reviews WHEN (SELECT secure_delete FROM pragma_secure_delete)=0 BEGIN SELECT RAISE(ABORT,'secure_delete off'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.etsyScrubReviewBuyerFields(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`SELECT data FROM resources WHERE resource_type='reviews'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	etsyAssertReviewClean(t, raw)
}
