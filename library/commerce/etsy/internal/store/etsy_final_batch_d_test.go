package store

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

// etsy-ext: final synthetic re-entry and incoming-only regressions.
func TestEtsyFinalFloorDependents(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		t.Run(map[bool]string{false: "receipt-first", true: "orphan-first"}[reversed], func(t *testing.T) {
			s := retentionStore(t)
			receipt := json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)
			transaction := json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001,"created_timestamp":100,"shipped_timestamp":150}`)
			payment := json.RawMessage(`{"payment_id":1000000004,"receipt_id":1000000001,"created_timestamp":100,"shipped_timestamp":150}`)
			if reversed {
				if e := s.UpsertTransactions(transaction); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true}); e != nil {
				t.Fatal(e)
			}
			if !reversed {
				if e := s.UpsertReceipts(receipt); e != nil {
					t.Fatal(e)
				}
			}
			if n := retentionCount(t, s, "SELECT count(*) FROM etsy_retention_receipts WHERE id='1000000001'"); n != 1 {
				t.Fatal("forgotten receipt", n)
			}
			if e := s.UpsertTransactions(transaction); e != nil {
				t.Fatal(e)
			}
			if e := s.UpsertPayments(payment); e != nil {
				t.Fatal(e)
			}
			if retentionCount(t, s, "SELECT count(*) FROM transactions") != 0 || retentionCount(t, s, "SELECT count(*) FROM payments") != 0 {
				t.Fatal("dependent re-entered")
			}
			for _, apply := range []bool{false, true} {
				r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, ClearFloor: true, Apply: apply})
				if e != nil || r.ReadmittedReceipts != 1 {
					t.Fatalf("readmission: %+v %v", r, e)
				}
			}
			if e := s.UpsertReceipts(receipt); e != nil {
				t.Fatal(e)
			}
			if e := s.UpsertTransactions(transaction); e != nil {
				t.Fatal(e)
			}
			if e := s.UpsertPayments(payment); e != nil {
				t.Fatal(e)
			}
			if retentionCount(t, s, "SELECT count(*) FROM receipts") != 1 || retentionCount(t, s, "SELECT count(*) FROM transactions") != 1 || retentionCount(t, s, "SELECT count(*) FROM payments") != 1 {
				t.Fatal("clear did not restore")
			}
		})
	}
}

func TestEtsyFinalOrphanVeto(t *testing.T) {
	s := retentionStore(t)
	for _, raw := range []string{`{"transaction_id":1000000003,"receipt_id":1000000001,"created_timestamp":100,"shipped_timestamp":150}`, `{"transaction_id":1000000004,"receipt_id":1000000002,"created_timestamp":100,"shipped_timestamp":null}`, `{"transaction_id":1000000005,"receipt_id":1000000002,"created_timestamp":100,"shipped_timestamp":150}`} {
		if e := s.UpsertTransactions(json.RawMessage(raw)); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM etsy_retention_receipts WHERE id='1000000002'") != 0 {
		t.Fatal("vetoed orphan remembered")
	}
	if retentionCount(t, s, "SELECT count(*) FROM transactions") != 1 {
		t.Fatal("vetoed orphan removed")
	}
}

func TestEtsyFinalShippedSelection(t *testing.T) {
	s := retentionStore(t)
	for _, raw := range []string{
		`{"receipt_id":1000000001,"created_timestamp":100}`,
		`{"receipt_id":1000000002,"created_timestamp":100,"is_shipped":true}`,
		`{"receipt_id":1000000003,"created_timestamp":300,"is_shipped":false}`,
		`{"receipt_id":1000000004,"created_timestamp":100,"is_shipped":false}`,
	} {
		if e := s.UpsertReceipts(json.RawMessage(raw)); e != nil {
			t.Fatal(e)
		}
	}
	// Conflicting projections: explicit false vetoes an otherwise shipped receipt.
	retentionExec(t, s, `UPDATE receipts SET is_shipped=0 WHERE receipt_id=1000000002`)
	for _, apply := range []bool{false, true} {
		r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: apply})
		if e != nil || r.KeptUnknownShipped != 1 || r.KeptUnshipped != 2 || r.Counts["receipts"] != 0 {
			t.Fatalf("selection: %+v %v", r, e)
		}
	}
}

func TestEtsyFinalLegacyReviewRejectedReplacement(t *testing.T) {
	s := retentionStore(t)
	legacy := `{"transaction_id":1000000003,"review":"synthetic legacy"}`
	retentionExec(t, s, `INSERT INTO resources(resource_type,id,data) VALUES('reviews','pp:params:synthetic',?)`, legacy)
	retentionExec(t, s, `INSERT INTO reviews(id,transaction_id,data) VALUES('pp:params:synthetic',1000000003,?)`, legacy)
	if _, e := s.EtsyPrune(EtsyPruneOptions{Reviews: true, Cutoff: 200, Apply: true}); e != nil {
		t.Fatal(e)
	}
	raw := json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100,"review":"synthetic refresh"}`)
	stored, _, _, purged, e := s.UpsertBatchDetailedPurgingReviews("reviews", []json.RawMessage{raw}, []string{"1000000003"})
	if e != nil || stored != 0 || purged != 0 {
		t.Fatalf("replacement %d %d %v", stored, purged, e)
	}
	if retentionCount(t, s, "SELECT count(*) FROM reviews WHERE id='pp:params:synthetic'") != 1 || retentionCount(t, s, "SELECT count(*) FROM resources WHERE resource_type='reviews' AND id='pp:params:synthetic'") != 1 {
		t.Fatal("legacy row deleted")
	}
}

func TestEtsyFinalGenericStorageID(t *testing.T) {
	s := retentionStore(t)
	if e := s.Upsert("reviews", "synthetic-caller-key", json.RawMessage(`{"review":"before"}`)); e != nil {
		t.Fatal(e)
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Reviews: true, Cutoff: 200, Apply: true}); e != nil {
		t.Fatal(e)
	}
	if e := s.Upsert("reviews", "synthetic-caller-key", json.RawMessage(`{"created_timestamp":100,"review":"after"}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources WHERE id='synthetic-caller-key' AND json_extract(data,'$.review')='after'`) != 1 {
		t.Fatal("stored update rejected")
	}
}

func TestEtsyFinalFilterReadOnly(t *testing.T) {
	s := retentionStore(t)
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true}); e != nil {
		t.Fatal(e)
	}
	// A separate writer may hold RESERVED; a read transaction must still proceed.
	writer, e := sql.Open("sqlite", s.path+"?_txlock=immediate&_pragma=busy_timeout(10)")
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Close()
	tx, e := writer.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	retentionExec(t, s, `PRAGMA busy_timeout=10`)
	done := make(chan error, 1)
	s.lockForWrite()
	go func() {
		_, n, e := s.FilterEtsyRetentionIncoming("receipts", []json.RawMessage{json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)})
		if e == nil && n != 1 {
			e = sql.ErrNoRows
		}
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
		t.Fatal("filter acquired write lock")
	}
	if retentionCount(t, s, "SELECT count(*) FROM etsy_retention_receipts") != 0 {
		t.Fatal("filter mutated metadata")
	}
}

func TestEtsyFinalReviewLinkWarning(t *testing.T) {
	s := retentionStore(t)
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100}`)); e != nil {
		t.Fatal(e)
	}
	for _, apply := range []bool{false, true} {
		r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: apply})
		if e != nil || r.ReviewsUnlinkableAfterPrune != 1 {
			t.Fatalf("link warning %+v %v", r, e)
		}
	}
}

func TestEtsyFinalGuestTransactionKinds(t *testing.T) {
	s := retentionStore(t)
	// Embedded transactions must remain suppressed even without standalone rows.
	receipt := json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"transactions":[{"transaction_id":1000000003}]}`)
	if e := s.UpsertReceipts(receipt); e != nil {
		t.Fatal(e)
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{ReceiptIDs: []string{"1000000001"}}}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"reviews", "listings_reviews", "transactions", "listings_transactions"} {
		if e := s.Upsert(kind, "1000000003", json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100}`)); e != nil {
			t.Fatal(e)
		}
	}
	if retentionCount(t, s, "SELECT count(*) FROM resources WHERE resource_type IN ('reviews','listings_reviews','transactions','listings_transactions')") != 0 {
		t.Fatal("guest transaction link re-entered")
	}
}
