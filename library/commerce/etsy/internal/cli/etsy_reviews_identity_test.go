package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsy-ext: sync removes legacy fingerprints from typed, generic, and FTS storage.
func TestEtsyReviewBatchPurgesLegacy(t *testing.T) {
	for _, dispatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "discriminator"}[dispatch], func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if dispatch {
				old, existed := discriminatorDispatchers["reviews"]
				d := discriminatorDispatchers["reviews"]
				d.Field = "unused"
				discriminatorDispatchers["reviews"] = d
				defer func() {
					if existed {
						discriminatorDispatchers["reviews"] = old
					} else {
						delete(discriminatorDispatchers, "reviews")
					}
				}()
			}
			raw := `{"shop_id":9000001,"listing_id":1000000001,"transaction_id":4000000001,"rating":4,"review":"Edited Juniper Ceramics mug"}`
			for _, q := range []string{
				`INSERT INTO reviews(id,data,transaction_id) VALUES('pp:params:abc',?,4000000001)`,
				`INSERT INTO resources(id,resource_type,data) VALUES('pp:params:abc','reviews',?)`,
			} {
				if _, err := db.DB().Exec(q, raw); err != nil {
					t.Fatal(err)
				}
			}
			// Use the store's normal upsert to seed a real legacy FTS entry.
			legacy := json.RawMessage(`{"shop_id":9000001,"review":"Juniper Ceramics legacy"}`)
			if _, _, _, err := db.UpsertBatchDetailed("reviews", []json.RawMessage{legacy}); err != nil {
				t.Fatal(err)
			}
			// Mark the hash row as carrying a transaction in both storage forms.
			if _, err := db.DB().Exec(`UPDATE reviews SET transaction_id=4000000001 WHERE id LIKE 'pp:params:%'`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().Exec(`UPDATE resources SET data=? WHERE resource_type='reviews'`, raw); err != nil {
				t.Fatal(err)
			}
			var before int
			if err := db.DB().QueryRow(`SELECT count(*) FROM resources_fts`).Scan(&before); err != nil || before == 0 {
				t.Fatalf("FTS seed: %d %v", before, err)
			}
			n, e, f, err := upsertResourceBatch(db, "reviews", []json.RawMessage{json.RawMessage(raw)}, io.Discard)
			if err != nil || n != 1 || e != 0 || f != 0 {
				t.Fatalf("batch: %d %d %d %v", n, e, f, err)
			}
			for _, q := range []string{
				`SELECT count(*) FROM reviews`,
				`SELECT count(*) FROM resources WHERE resource_type='reviews'`,
				`SELECT count(*) FROM reviews WHERE id='4000000001'`,
				`SELECT count(*) FROM resources_fts`,
			} {
				var n int
				if err := db.DB().QueryRow(q).Scan(&n); err != nil || n != 1 {
					t.Fatalf("remaining rows: %d %v", n, err)
				}
			}
		})
	}
}

func TestEtsyReviewBatchKeepsUnbatchedLegacy(t *testing.T) {
	etsyOldHuman := humanFriendly
	humanFriendly = false
	t.Cleanup(func() { humanFriendly = etsyOldHuman })
	for _, etsyDispatch := range []bool{false, true} {
		t.Run(fmt.Sprint(etsyDispatch), func(t *testing.T) {
			if etsyDispatch {
				etsyOld, etsyExists := discriminatorDispatchers["reviews"]
				discriminatorDispatchers["reviews"] = discriminatorDispatch{Field: "unused"}
				t.Cleanup(func() {
					if etsyExists {
						discriminatorDispatchers["reviews"] = etsyOld
					} else {
						delete(discriminatorDispatchers, "reviews")
					}
				})
			}
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			etsySeed := func(etsyTx int64) {
				etsyRaw := fmt.Sprintf(`{"transaction_id":%d,"review":"Juniper Ceramics"}`, etsyTx)
				etsyID := fmt.Sprintf("pp:params:%d", etsyTx)
				if _, err := db.DB().Exec(`INSERT INTO reviews(id,data,transaction_id) VALUES(?,?,?)`, etsyID, etsyRaw, etsyTx); err != nil {
					t.Fatal(err)
				}
				if _, err := db.DB().Exec(`INSERT INTO resources(id,resource_type,data) VALUES(?,'reviews',?)`, etsyID, etsyRaw); err != nil {
					t.Fatal(err)
				}
			}
			etsySeed(4000000001)
			etsySeed(4000000002)
			for _, etsyMatching := range []bool{false, true} {
				if etsyMatching {
					etsySeed(4000000003)
				}
				var etsyEvents bytes.Buffer
				_, _, _, err := upsertResourceBatch(db, "reviews", []json.RawMessage{json.RawMessage(`{"transaction_id":4000000003,"review":"Juniper Ceramics"}`)}, &etsyEvents)
				if err != nil {
					t.Fatal(err)
				}
				for _, etsyQuery := range []string{
					`SELECT count(*) FROM reviews`,
					`SELECT count(*) FROM resources WHERE resource_type='reviews'`,
				} {
					var n int
					if err := db.DB().QueryRow(etsyQuery).Scan(&n); err != nil || n != 3 {
						t.Fatalf("%s: %d %v", etsyQuery, n, err)
					}
				}
				for _, etsyTable := range []string{"reviews", "resources"} {
					for _, etsyID := range []string{"pp:params:4000000001", "pp:params:4000000002", "4000000003"} {
						var n int
						if err := db.DB().QueryRow("SELECT count(*) FROM "+etsyTable+" WHERE id=?", etsyID).Scan(&n); err != nil || n != 1 {
							t.Fatalf("%s %s: %d %v", etsyTable, etsyID, n, err)
						}
					}
				}
				if etsyMatching {
					var etsyEvent struct {
						Reason string
						Purged int
					}
					if err := json.Unmarshal(bytes.TrimSpace(etsyEvents.Bytes()), &etsyEvent); err != nil || etsyEvent.Reason != "legacy_reviews_purged" || etsyEvent.Purged != 1 {
						t.Fatalf("event: %s %v", etsyEvents.String(), err)
					}
				} else if bytes.Contains(etsyEvents.Bytes(), []byte("legacy_reviews_purged")) {
					t.Fatalf("unexpected event: %s", etsyEvents.String())
				}
			}
		})
	}
}

func TestEtsyReviewTransactionIDs(t *testing.T) {
	etsyIDs := etsyReviewTransactionIDs([]json.RawMessage{
		json.RawMessage(`{"transaction_id":4000000001}`), json.RawMessage(`{"transaction_id":"4000000001"}`),
		json.RawMessage(`{"transaction_id":"4000000002"}`), json.RawMessage(`{"transaction_id":null}`),
		json.RawMessage(`{}`), json.RawMessage(`{"transaction_id":"bad"}`), json.RawMessage(`[]`),
	})
	if fmt.Sprint(etsyIDs) != "[4000000001 4000000002]" {
		t.Fatal(etsyIDs)
	}
}
