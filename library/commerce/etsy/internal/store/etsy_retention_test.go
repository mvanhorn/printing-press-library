package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func retentionStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(filepath.Join(t.TempDir(), "data.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func retentionExec(t *testing.T, s *Store, q string, args ...any) {
	t.Helper()
	if _, e := s.db.Exec(q, args...); e != nil {
		t.Fatal(e)
	}
}
func retentionCount(t *testing.T, s *Store, q string, args ...any) int {
	t.Helper()
	var n int
	if e := s.db.QueryRow(q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func TestEtsyRetentionCascade(t *testing.T) {
	s := retentionStore(t)
	receipt := `{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true,"transactions":[{"transaction_id":1000000002}]}`
	if e := s.UpsertReceipts(json.RawMessage(receipt)); e != nil {
		t.Fatal(e)
	}
	if e := s.RecordPaymentCheck("1000000001", sql.NullInt64{Int64: 100, Valid: true}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"receipts", "transactions", "payments", "shops_receipts_payments", "listings_transactions", "receipts_listings"} {
		data := `{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`
		for _, id := range []string{"1000000002", "1000000002\x001000000001", "pp:params:synthetic"} {
			if e := s.Upsert(kind, id, json.RawMessage(data)); e != nil {
				t.Fatal(e)
			}
			if kind == "transactions" || kind == "payments" {
				retentionExec(t, s, `INSERT INTO "`+kind+`"(id,data,receipt_id) VALUES(?,?,?)`, id, data, "1000000001")
			}
			// Both historical FTS forms must disappear.
			retentionExec(t, s, `INSERT OR REPLACE INTO resources_fts(rowid,id,resource_type,content) VALUES(?,?,?,?)`, ftsRowID(kind, BareResourceID(id)), BareResourceID(id), kind, "ceramic mug")
		}
	}
	// A generic NUL form with no receipt reference follows its typed bare row.
	retentionExec(t, s, `INSERT INTO transactions(id,data,receipt_id) VALUES(?,?,?)`, "1000000010", `{}`, "1000000001")
	if e := s.Upsert("transactions", "1000000010\x001000000001", json.RawMessage(`{"transaction_id":1000000010}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.Upsert("transactions", "pp:params:counterpart", json.RawMessage(`{"transaction_id":1000000010}`)); e != nil {
		t.Fatal(e)
	}
	// Typed-only parent, generic-only parent, explicitly unshipped and missing age.
	retentionExec(t, s, `INSERT INTO receipts(id,data,receipt_id,created_timestamp,is_shipped) VALUES('1000000003','{}',1000000003,100,1)`)
	for id, data := range map[string]string{"1000000004": `{"receipt_id":1000000004,"create_timestamp":100,"is_shipped":true}`, "1000000005": `{"receipt_id":1000000005,"created_timestamp":100,"is_shipped":false}`, "1000000006": `{"receipt_id":1000000006,"is_shipped":true}`} {
		if e := s.Upsert("receipts", id, json.RawMessage(data)); e != nil {
			t.Fatal(e)
		}
	}
	for _, kind := range []string{"reviews", "listings_reviews"} {
		if e := s.Upsert(kind, "1000000007", json.RawMessage(`{"transaction_id":1000000007,"create_timestamp":100,"review":"linen apron"}`)); e != nil {
			t.Fatal(e)
		}
	}
	o := EtsyPruneOptions{Cutoff: 200, Receipts: true, Reviews: true}
	preview, e := s.EtsyPrune(o)
	if e != nil {
		t.Fatal(e)
	}
	if preview.KeptNoTimestamp != 1 || preview.KeptUnshipped != 1 {
		t.Fatalf("%+v", preview)
	}
	if retentionCount(t, s, `SELECT count(*) FROM transactions`) != 4 {
		t.Fatal("preview deleted")
	}
	o.Apply = true
	applied, e := s.EtsyPrune(o)
	if e != nil {
		t.Fatal(e)
	}
	if !applied.Committed {
		t.Fatal("apply did not commit")
	}
	applied.Committed = false // Selection parity excludes execution status.
	if !reflect.DeepEqual(preview, applied) {
		t.Fatalf("preview=%+v apply=%+v", preview, applied)
	}
	for _, table := range []string{"transactions", "payments", "etsy_payment_checks"} {
		if retentionCount(t, s, `SELECT count(*) FROM "`+table+`"`) != 0 {
			t.Fatal(table)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 2 || retentionCount(t, s, `SELECT count(*) FROM resources_fts`) != 2 {
		t.Fatal("generic/FTS cascade incomplete")
	}
	for _, id := range []string{"1000000005", "1000000006"} {
		if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts' AND id=?`, id) != 1 {
			t.Fatal("wrong retained receipt", id)
		}
	}
	if ok, e := s.PaymentsEverSucceeded(); e != nil || !ok {
		t.Fatalf("lifetime success lost %v %v", ok, e)
	}
	if e := s.UpsertReceipts(json.RawMessage(receipt)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 0 {
		t.Fatal("floor reinserted receipt")
	}
	for _, kind := range []string{"transactions", "shops_receipts_payments", "listings_transactions", "receipts_listings"} {
		if e := s.Upsert(kind, "1000000008", json.RawMessage(`{"receipt_id":1000000001}`)); e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 2 {
		t.Fatal("late dependents reinserted")
	}
	_, e = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Reviews: true, ClearFloor: true, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.UpsertReceipts(json.RawMessage(receipt)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 1 {
		t.Fatal("clear did not reallow")
	}
}

func TestEtsyRetentionFTSErrorRollsBack(t *testing.T) {
	s := retentionStore(t)
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	retentionExec(t, s, `DROP TABLE resources_fts`)
	_, e := s.EtsyPrune(EtsyPruneOptions{Cutoff: 200, Receipts: true, Apply: true})
	if e == nil {
		t.Fatal("FTS error ignored")
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 1 {
		t.Fatal("partial delete")
	}
}

func TestEtsyRetentionReviewFloorAndReceiptScope(t *testing.T) {
	s := retentionStore(t)
	for id, shipped := range map[string]bool{"1000000001": true, "1000000002": false} {
		data := json.RawMessage(`{"receipt_id":` + id + `,"created_timestamp":100,"is_shipped":` + map[bool]string{true: "true", false: "false"}[shipped] + `}`)
		if e := s.UpsertReceipts(data); e != nil {
			t.Fatal(e)
		}
	}
	for _, id := range []string{"1000000001", "1000000002"} {
		if e := s.Upsert("receipts_listings", "1000000003\x00"+id, json.RawMessage(`{"listing_id":1000000003,"receipt_id":`+id+`}`)); e != nil {
			t.Fatal(e)
		}
	}
	_, e := s.EtsyPrune(EtsyPruneOptions{Cutoff: 200, Receipts: true, Reviews: true, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts_listings'`) != 1 {
		t.Fatal("cross-receipt listing was pruned")
	}
	for _, kind := range []string{"reviews", "listings_reviews"} {
		_, _, _, e = s.UpsertBatchDetailed(kind, []json.RawMessage{json.RawMessage(`{"transaction_id":1000000004,"create_timestamp":100}`)})
		if e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type IN ('reviews','listings_reviews')`) != 0 {
		t.Fatal("review floor bypassed")
	}
	if e = s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000005,"created_timestamp":100}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM reviews`) != 0 {
		t.Fatal("typed review floor bypassed")
	}
}

func TestEtsyRetentionTimestampBoundary(t *testing.T) {
	s := retentionStore(t)
	_, e := s.EtsyPrune(EtsyPruneOptions{Cutoff: 200, Reviews: true, Receipts: true, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, age := range []string{"0", "199", "200", "null"} {
		if e = s.Upsert("reviews", age, json.RawMessage(`{"create_timestamp":`+age+`}`)); e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='reviews'`) != 2 {
		t.Fatal("floor timestamp boundary")
	}
	if e = s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":false}`)); e != nil {
		t.Fatal(e)
	}
	if e = s.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000002,"receipt_id":1000000001,"created_timestamp":100}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM transactions`) != 1 {
		t.Fatal("unshipped dependent dropped")
	}
}

func TestEtsyRetentionLegacyChecks(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, `CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY,receipt_updated INTEGER,checked_at INTEGER)`)
	retentionExec(t, s, `INSERT INTO etsy_payment_checks VALUES('1000000001',100,100)`)
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"create_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	_, e := s.EtsyPrune(EtsyPruneOptions{Cutoff: 200, Receipts: true, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	if ok, e := s.PaymentsEverSucceeded(); e != nil || !ok {
		t.Fatal("legacy success lost", e)
	}
}

func TestEtsyRetentionStringIDsAndAbsentChecks(t *testing.T) {
	s := retentionStore(t)
	for _, kind := range []string{"receipts", "transactions", "etsy_payment_checks"} {
		id := "pp:params:" + kind
		if e := s.Upsert(kind, id, json.RawMessage(`{"receipt_id":"1000000001","transaction_id":"1000000002","created_timestamp":100,"is_shipped":true}`)); e != nil {
			t.Fatal(e)
		}
	}
	_, e := s.EtsyPrune(EtsyPruneOptions{Cutoff: 200, Receipts: true, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 0 {
		t.Fatal("string id or absent-checks cascade")
	}
}

// etsy-ext: shipped updates persist until the next explicit prune.
func TestEtsyRetentionShippedUpdateAwaitsPrune(t *testing.T) {
	s := retentionStore(t)
	for i := 0; i < 2; i++ {
		rid, tid := 1000000001+i, 1000000011+i
		if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"buyer_user_id":1000000099,"created_timestamp":100,"is_shipped":false}`, rid))); e != nil {
			t.Fatal(e)
		}
		if e := s.UpsertTransactions(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d}`, tid, rid))); e != nil {
			t.Fatal(e)
		}
		if e := s.UpsertReviews(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"created_timestamp":500}`, tid))); e != nil {
			t.Fatal(e)
		}
		if _, _, e := s.UpsertBatch("payments", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"payment_id":%d,"receipt_id":%d}`, 1000000021+i, rid))}); e != nil {
			t.Fatal(e)
		}
		if e := s.Upsert("receipts_listings", fmt.Sprint(1000000031)+string(rune(0))+fmt.Sprint(rid), json.RawMessage(fmt.Sprintf(`{"listing_id":1000000031,"parent_id":%d}`, rid))); e != nil {
			t.Fatal(e)
		}
	}
	retentionExec(t, s, `INSERT INTO search_learnings(query_pattern,resource_type,resource_id,action,source) VALUES('synthetic receipt','receipts','1000000001','fetch','taught'),('synthetic transaction','transactions','1000000011','fetch','taught'),('other receipt','receipts','1000000002','fetch','taught')`)
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true})
	if e != nil || r.KeptUnshipped != 2 {
		t.Fatalf("%+v %v", r, e)
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM receipts WHERE receipt_id=1000000001 AND is_shipped=1") != 1 {
		t.Fatal("stored shipped update rejected")
	}
	if retentionCount(t, s, "SELECT count(*) FROM transactions") != 2 {
		t.Fatal("write deleted dependents")
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"receipts", "transactions", "payments", "receipts_listings"} {
		if kind != "receipts_listings" && retentionCount(t, s, `SELECT count(*) FROM "`+kind+`"`) != 1 {
			t.Fatalf("typed cascade %s", kind)
		}
		if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type=?`, kind) != 1 {
			t.Fatalf("projection cascade %s", kind)
		}
	}
	for table, condition := range map[string]string{"receipts": "receipt_id=1000000002", "transactions": "transaction_id=1000000012", "reviews": "transaction_id=1000000012", "payments": "payment_id=1000000022"} {
		if retentionCount(t, s, `SELECT count(*) FROM "`+table+`" WHERE `+condition) != 1 {
			t.Fatalf("other receipt's %s lost", table)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts_listings' AND json_extract(data,'$.parent_id')=1000000002`) != 1 {
		t.Fatal("shared listing association lost")
	}
	if retentionCount(t, s, `SELECT count(*) FROM search_learnings`) != 1 {
		t.Fatal("prune left victim learnings")
	}
	if retentionCount(t, s, `SELECT count(*) FROM etsy_retention_receipts WHERE id='1000000001'`) != 1 {
		t.Fatal("floor receipt unrecorded")
	}
}

func TestEtsyRetentionParentAgeAndFloors(t *testing.T) {
	s := retentionStore(t)
	for _, cutoff := range []int64{200, 100} {
		if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Reviews: true, Apply: true, Cutoff: cutoff}); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":150,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 0 {
		t.Fatal("floor decreased")
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000002,"created_timestamp":5000,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.Upsert("receipts_listings", "1000000003\x001000000002", json.RawMessage(`{"listing_id":1000000003,"parent_id":1000000002,"created_timestamp":150}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts_listings'`) != 1 {
		t.Fatal("dependent's own age used")
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, ClearFloor: true, Apply: true}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM etsy_retention_floor WHERE scope='reviews' AND cutoff=200`) != 1 {
		t.Fatal("reviews floor cleared")
	}
	if e := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000004,"created_timestamp":150}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM reviews`) != 0 {
		t.Fatal("reviews floor ignored")
	}
}

func TestEtsyRetentionAgeBoundary(t *testing.T) {
	s := retentionStore(t)
	for i, age := range []int{199, 200, 201} {
		rid, tid := 1000000001+i*10, 1000000002+i*10
		if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":%d,"is_shipped":true}`, rid, age))); e != nil {
			t.Fatal(e)
		}
		if e := s.UpsertTransactions(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d}`, tid, rid))); e != nil {
			t.Fatal(e)
		}
		if e := s.UpsertPayments(json.RawMessage(fmt.Sprintf(`{"payment_id":%d,"receipt_id":%d}`, tid+1, rid))); e != nil {
			t.Fatal(e)
		}
		if e := s.UpsertReviews(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"created_timestamp":%d}`, tid, age))); e != nil {
			t.Fatal(e)
		}
	}
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Reviews: true, Cutoff: 200, Apply: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"receipts", "transactions", "payments", "reviews"} {
		if r.Counts[kind] != 1 || retentionCount(t, s, `SELECT count(*) FROM "`+kind+`"`) != 2 {
			t.Fatalf("boundary %s: %+v", kind, r)
		}
	}
	for _, id := range []int64{1000000011, 1000000021} {
		if retentionCount(t, s, `SELECT count(*) FROM receipts WHERE receipt_id=?`, id) != 1 {
			t.Fatal("wrong survivor", id)
		}
	}
}

func TestEtsyRetentionListingsTakesNoWriteLock(t *testing.T) {
	s := retentionStore(t)
	s.lockForWrite()
	done := make(chan error, 1)
	go func() {
		_, _, e := s.FilterEtsyRetentionIncoming("listings", []json.RawMessage{json.RawMessage(`{"listing_id":1000000001}`)})
		done <- e
	}()
	select {
	case e := <-done:
		s.unlockAfterWrite()
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		s.unlockAfterWrite()
		<-done
		t.Fatal("non-buyer filter took write lock")
	}
}

func TestEtsyRetentionBoundaryProjections(t *testing.T) {
	for _, params := range []bool{false, true} {
		t.Run(fmt.Sprint(params), func(t *testing.T) {
			s := retentionStore(t)
			for i, age := range []int{199, 200, 201} {
				rid, tid, pid := 1000000001+i*10, 1000000002+i*10, 1000000003+i*10
				for kind, raw := range map[string]string{
					"receipts":     fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":%d,"is_shipped":true}`, rid, age),
					"transactions": fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d}`, tid, rid),
					"payments":     fmt.Sprintf(`{"payment_id":%d,"receipt_id":%d}`, pid, rid),
					"reviews":      fmt.Sprintf(`{"transaction_id":%d,"created_timestamp":%d}`, tid, age),
				} {
					id := fmt.Sprint(rid)
					if params {
						id = "pp:params:" + fmt.Sprint(rid)
					}
					if e := s.Upsert(kind, id, json.RawMessage(raw)); e != nil {
						t.Fatal(e)
					}
				}
			}
			r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Reviews: true, Apply: true, Cutoff: 200})
			if e != nil {
				t.Fatal(e)
			}
			for _, kind := range []string{"receipts", "transactions", "payments", "reviews"} {
				if r.Counts[kind] != 1 || retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type=?`, kind) != 2 {
					t.Fatalf("%s: %+v", kind, r)
				}
			}
			for _, id := range []int64{1000000011, 1000000021} {
				if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts' AND CASE WHEN json_valid(data) THEN json_extract(data,'$.receipt_id') END=?`, id) != 1 {
					t.Fatal("wrong survivor", id)
				}
			}
		})
	}
}
