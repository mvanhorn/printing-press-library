package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// etsy-ext: transaction identity survives review edits; missing IDs keep hash fallback.
func TestEtsyReviewIdentity(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if n, err := db.PurgeParamKeyedReviews(nil); err != nil || n != 0 {
		t.Fatalf("empty purge: %d %v", n, err)
	}
	for _, raw := range []string{
		`{"shop_id":9000001,"listing_id":1000000001,"transaction_id":4000000001,"rating":5,"review":"Lovely Juniper Ceramics mug"}`,
		`{"shop_id":9000001,"listing_id":1000000001,"transaction_id":4000000001,"rating":4,"review":"Edited Juniper Ceramics mug"}`,
	} {
		n, e, f, err := db.UpsertBatchDetailed("reviews", []json.RawMessage{json.RawMessage(raw)})
		if err != nil || n != 1 || e != 0 || f != 0 {
			t.Fatalf("upsert: %d %d %d %v", n, e, f, err)
		}
	}
	for _, q := range []string{`SELECT count(*) FROM reviews WHERE id='4000000001' AND rating=4`, `SELECT count(*) FROM resources WHERE resource_type='reviews'`} {
		var n int
		if err := db.db.QueryRow(q).Scan(&n); err != nil || n != 1 {
			t.Fatalf("identity count: %d %v", n, err)
		}
	}
	raw := json.RawMessage(`{"shop_id":9000001,"listing_id":1000000001,"rating":5,"review":"Juniper Ceramics"}`)
	n, e, f, err := db.UpsertBatchDetailed("reviews", []json.RawMessage{raw})
	if err != nil || n != 1 || e != 0 || f != 0 {
		t.Fatalf("hash upsert: %d %d %d %v", n, e, f, err)
	}
	if n, err := db.PurgeParamKeyedReviews([]string{"4000000001"}); err != nil || n != 0 {
		t.Fatalf("hash purge: %d %v", n, err)
	}
	var hashes int
	if err := db.db.QueryRow(`SELECT count(*) FROM reviews WHERE id LIKE 'pp:params:%'`).Scan(&hashes); err != nil || hashes != 1 {
		t.Fatalf("hash count: %d %v", hashes, err)
	}
}

func TestEtsyPurgeScopedToBatch(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i, id := range []string{"pp:params:a", "pp:params:b", "pp:params:c"} {
		raw := fmt.Sprintf(`{"transaction_id":%d,"review":"Juniper Ceramics"}`, 4000000001+i)
		if _, err := db.db.Exec(`INSERT INTO reviews(id,data,transaction_id) VALUES(?,?,?)`, id, raw, 4000000001+i); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.Exec(`INSERT INTO resources(id,resource_type,data) VALUES(?,'reviews',?)`, id, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.Exec(`INSERT INTO resources_fts(rowid,resource_type,id,content) VALUES(?,'reviews',?,?)`, ftsRowID("reviews", id), id, "Juniper Ceramics"); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := db.PurgeParamKeyedReviews([]string{"4000000003", "4000000003"}); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	for i, id := range []string{"pp:params:a", "pp:params:b", "pp:params:c"} {
		want := 1
		if i == 2 {
			want = 0
		}
		for _, q := range []string{`SELECT count(*) FROM reviews WHERE id=?`, `SELECT count(*) FROM resources WHERE resource_type='reviews' AND id=?`, `SELECT count(*) FROM resources_fts WHERE id=?`} {
			var n int
			if err := db.db.QueryRow(q, id).Scan(&n); err != nil || n != want {
				t.Fatalf("%s %s: %d want %d: %v", q, id, n, want, err)
			}
		}
	}
}

func TestEtsyPurgeSingleStorage(t *testing.T) {
	for _, table := range []string{"reviews", "resources"} {
		t.Run(table, func(t *testing.T) {
			db, err := Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			query := `INSERT INTO reviews(id,data,transaction_id) VALUES('pp:params:x','{"transaction_id":4000000001,"review":"Juniper Ceramics"}',4000000001)`
			if table == "resources" {
				query = `INSERT INTO resources(id,resource_type,data) VALUES('pp:params:x','reviews','{"transaction_id":4000000001,"review":"Juniper Ceramics"}')`
			}
			if _, err := db.db.Exec(query); err != nil {
				t.Fatal(err)
			}
			if _, err := db.db.Exec(`INSERT INTO resources_fts(rowid,resource_type,id,content) VALUES(?,'reviews','pp:params:x','Juniper Ceramics')`, ftsRowID("reviews", "pp:params:x")); err != nil {
				t.Fatal(err)
			}
			if n, err := db.PurgeParamKeyedReviews([]string{"4000000001"}); err != nil || n != 1 {
				t.Fatalf("purge: %d %v", n, err)
			}
			for _, name := range []string{"reviews", "resources", "resources_fts"} {
				var n int
				if err := db.db.QueryRow("SELECT count(*) FROM " + name + " WHERE id='pp:params:x'").Scan(&n); err != nil || n != 0 {
					t.Fatalf("%s: %d %v", name, n, err)
				}
			}
		})
	}
}

func TestEtsyUpsertBatchPurgingReviewsAtomic(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw := json.RawMessage(`{"transaction_id":4000000001,"review":"Juniper Ceramics"}`)
	for _, query := range []string{
		`INSERT INTO reviews(id,data,transaction_id) VALUES('pp:params:x',?,4000000001)`,
		`INSERT INTO resources(id,resource_type,data) VALUES('pp:params:x','reviews',?)`,
	} {
		if _, err := db.db.Exec(query, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.db.Exec(`INSERT INTO resources_fts(rowid,resource_type,id,content) VALUES(?,'reviews','pp:params:x','Juniper Ceramics')`, ftsRowID("reviews", "pp:params:x")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`CREATE TRIGGER etsy_test_fail BEFORE INSERT ON resources WHEN NEW.resource_type='reviews' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END;`); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{true, false} {
		if !fail {
			if _, err := db.db.Exec(`DROP TRIGGER etsy_test_fail`); err != nil {
				t.Fatal(err)
			}
		}
		stored, extracted, typed, purged, err := db.UpsertBatchDetailedPurgingReviews("reviews", []json.RawMessage{raw}, []string{"4000000001"})
		wantLegacy, wantNew := 0, 1
		if fail {
			wantLegacy, wantNew = 1, 0
			if err == nil || purged != 0 {
				t.Fatalf("rollback: purged=%d err=%v", purged, err)
			}
		} else if err != nil || stored != 1 || extracted != 0 || typed != 0 || purged != 1 {
			t.Fatalf("success: %d %d %d %d %v", stored, extracted, typed, purged, err)
		}
		for _, table := range []string{"reviews", "resources", "resources_fts"} {
			for id, want := range map[string]int{"pp:params:x": wantLegacy, "4000000001": wantNew} {
				var n int
				if err := db.db.QueryRow("SELECT count(*) FROM "+table+" WHERE id=?", id).Scan(&n); err != nil || n != want {
					t.Fatalf("%s %s: %d want %d: %v", table, id, n, want, err)
				}
			}
		}
	}
}
