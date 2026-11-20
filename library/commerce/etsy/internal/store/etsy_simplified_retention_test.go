package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// etsy-ext: synthetic regression cases for incoming-only retention.
func TestEtsySimplifiedReviewsAndNeverStored(t *testing.T) {
	s := retentionStore(t)
	for _, kind := range []string{"reviews", "listings_reviews"} {
		if e := s.Upsert(kind, "1000000003", json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100,"review":"before"}`)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM resources WHERE resource_type IN ('reviews','listings_reviews')") != 2 {
		t.Fatal("receipt prune touched reviews")
	}
	for _, kind := range []string{"reviews", "listings_reviews"} {
		if e := s.Upsert(kind, "1000000003", json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100,"review":"after"}`)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Reviews: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"reviews", "listings_reviews"} {
		if e := s.Upsert(kind, "1000000003", json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100}`)); e != nil {
			t.Fatal(e)
		}
		if e := s.Upsert(kind, "1000000004", json.RawMessage(`{"transaction_id":1000000004,"created_timestamp":300}`)); e != nil {
			t.Fatal(e)
		}
		if e := s.Upsert(kind, "1000000004", json.RawMessage(`{"transaction_id":1000000004,"created_timestamp":100,"review":"updated"}`)); e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, "SELECT count(*) FROM resources WHERE resource_type IN ('reviews','listings_reviews') AND json_extract(data,'$.review')='updated'") != 2 {
		t.Fatal("stored review update rejected")
	}
	for _, shipped := range []bool{false, true} {
		if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":100,"is_shipped":%t}`, 1000000010+map[bool]int{false: 0, true: 1}[shipped], shipped))); e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, "SELECT count(*) FROM receipts") != 1 {
		t.Fatal("never stored shipped/unshipped policy")
	}
}

func TestEtsySimplifiedUnsuppressRestores(t *testing.T) {
	for _, receiptOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(receiptOnly), func(t *testing.T) {
			s := retentionStore(t)
			buyer := 1000000002
			if receiptOnly {
				buyer = 0
			}
			receipt := json.RawMessage(fmt.Sprintf(`{"receipt_id":1000000001,"buyer_user_id":%d,"created_timestamp":100,"is_shipped":true}`, buyer))
			transaction := json.RawMessage(fmt.Sprintf(`{"transaction_id":1000000003,"receipt_id":1000000001,"buyer_user_id":%d,"created_timestamp":100}`, buyer))
			review := json.RawMessage(fmt.Sprintf(`{"transaction_id":1000000003,"buyer_user_id":%d,"created_timestamp":100}`, buyer))
			sync := func() {
				t.Helper()
				for kind, raw := range map[string]json.RawMessage{"receipts": receipt, "transactions": transaction, "reviews": review} {
					if _, _, e := s.UpsertBatch(kind, []json.RawMessage{raw}); e != nil {
						t.Fatal(e)
					}
				}
			}
			sync()
			if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{ReceiptIDs: []string{"1000000001"}}}); e != nil {
				t.Fatal(e)
			}
			sync()
			for _, kind := range []string{"receipts", "transactions", "reviews"} {
				if retentionCount(t, s, "SELECT count(*) FROM "+kind) != 0 {
					t.Fatal("suppressed row re-entered", kind)
				}
			}
			o := &EtsyEraseOptions{Unsuppress: []string{"1000000002"}}
			if receiptOnly {
				o = &EtsyEraseOptions{UnsuppressReceipts: []string{"1000000001"}}
			}
			if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: o}); e != nil {
				t.Fatal(e)
			}
			sync()
			for _, kind := range []string{"receipts", "transactions", "reviews"} {
				if retentionCount(t, s, "SELECT count(*) FROM "+kind) != 1 {
					t.Fatal("restore failed", kind)
				}
			}
		})
	}
}

func TestEtsySimplifiedAssociationAndOrphan(t *testing.T) {
	for _, erase := range []bool{false, true} {
		t.Run(fmt.Sprint(erase), func(t *testing.T) {
			s := retentionStore(t)
			if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)); e != nil {
				t.Fatal(e)
			}
			for _, kind := range []string{"receipts_listings", "shops_receipts_payments"} {
				retentionExec(t, s, "INSERT INTO resources(resource_type,id,data) VALUES(?,?,?)", kind, "1000000003\x001000000001", "{}")
			}
			if e := s.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000009,"receipt_id":1000000099,"created_timestamp":100,"shipped_timestamp":null}`)); e != nil {
				t.Fatal(e)
			}
			o := EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}
			if erase {
				o.Erase = &EtsyEraseOptions{ReceiptIDs: []string{"1000000001"}}
			}
			if _, e := s.EtsyPrune(o); e != nil {
				t.Fatal(e)
			}
			if retentionCount(t, s, "SELECT count(*) FROM resources WHERE resource_type IN ('receipts_listings','shops_receipts_payments')") != 0 {
				t.Fatal("NUL association retained")
			}
			if retentionCount(t, s, "SELECT count(*) FROM transactions") != 1 {
				t.Fatal("unshipped orphan removed")
			}
		})
	}
}

func TestEtsySimplifiedExplicitVacuumRerun(t *testing.T) {
	s := retentionStore(t)
	o := EtsyPruneOptions{Apply: true, Receipts: true, Vacuum: true, ExplicitVacuum: true, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}}
	for i := 0; i < 2; i++ {
		r, e := s.EtsyPrune(o)
		if e != nil || r.Vacuum != "completed" || r.VacuumSteps["vacuum"] != "completed" {
			t.Fatalf("%+v %v", r, e)
		}
	}
}

func TestEtsySimplifiedScale(t *testing.T) {
	s := retentionStore(t)
	var receipts, transactions []json.RawMessage
	for i := 0; i < 4000; i++ {
		receipts = append(receipts, json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":100,"is_shipped":true}`, 1000000000+i)))
		transactions = append(transactions, json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d,"created_timestamp":100}`, 1000010000+i, 1000000000+i)))
	}
	for kind, items := range map[string][]json.RawMessage{"receipts": receipts, "transactions": transactions} {
		if _, _, e := s.UpsertBatch(kind, items); e != nil {
			t.Fatal(e)
		}
	}
	start := time.Now()
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200})
	if e != nil || r.Counts["receipts"] != 4000 || r.Counts["transactions"] != 4000 {
		t.Fatalf("%+v %v", r, e)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("first preview: %v", elapsed)
	}
	if _, e = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	start = time.Now()
	for kind, items := range map[string][]json.RawMessage{"receipts": receipts[:2000], "transactions": transactions[:2000]} {
		kept, n, e := s.FilterEtsyRetentionIncoming(kind, items)
		if e != nil || n != 2000 || len(kept) != 0 {
			t.Fatalf("%s %d %v", kind, n, e)
		}
		if _, _, e = s.UpsertBatch(kind, items); e != nil {
			t.Fatal(e)
		}
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("pruned re-sync: %v", elapsed)
	}
	if retentionCount(t, s, "SELECT count(*) FROM receipts")+retentionCount(t, s, "SELECT count(*) FROM transactions") != 0 {
		t.Fatal("pruned rows returned")
	}
}

func TestEtsySimplifiedLearnCleanupScope(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, "CREATE TABLE IF NOT EXISTS learn_recall_misses(id INTEGER PRIMARY KEY)")
	retentionExec(t, s, "INSERT INTO learn_recall_misses(id) VALUES(991)")
	retentionExec(t, s, `INSERT INTO search_learnings(query_pattern,resource_type,resource_id,action,source) VALUES('receipt','receipts','1000000001','fetch','taught'),('review','reviews','1000000001','fetch','taught')`)
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM learn_recall_misses WHERE id=991") != 1 {
		t.Fatal("ordinary sync cleaned misses")
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM search_learnings WHERE resource_type='reviews'") != 1 {
		t.Fatal("same id in another resource lost")
	}
	if retentionCount(t, s, "SELECT count(*) FROM search_learnings WHERE resource_type='receipts'") != 0 {
		t.Fatal("victim learning remains")
	}
}

// etsy-ext: reclaiming an empty prune reports absent external FTS honestly.
func TestEtsySimplifiedEmptyPruneVacuum(t *testing.T) {
	s := retentionStore(t)
	rows, e := s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND sql LIKE 'CREATE VIRTUAL TABLE%'")
	if e != nil {
		t.Fatal(e)
	}
	var names []string
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			t.Fatal(e)
		}
		names = append(names, name)
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	rows.Close()
	for _, name := range names {
		retentionExec(t, s, "DROP TABLE "+name)
	}
	r, e := s.EtsyPrune(EtsyPruneOptions{Apply: true, Receipts: true, Vacuum: true, Cutoff: 200})
	if e != nil || r.Vacuum != "completed" || r.VacuumSteps["rebuild"] != "not_applicable" {
		t.Fatalf("%+v %v", r, e)
	}
}

func TestEtsySimplifiedUnsuppressedCount(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, "INSERT INTO resources(resource_type,id,data) VALUES('receipts','1000000001','{\"buyer_user_id\":1000000002,\"created_timestamp\":100,\"is_shipped\":true}')")
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000003,"created_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200})
	if e != nil || r.UnsuppressedReceipts != 1 {
		t.Fatalf("%+v %v", r, e)
	}
	r, e = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{ReceiptIDs: []string{"1000000003"}}})
	if e != nil || r.UnsuppressedReceipts != 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
