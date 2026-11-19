package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// etsy-ext: receipts retention leaves all reviews untouched.
func TestEtsyRound2ReviewCycle(t *testing.T) {
	s := retentionStore(t)
	review := json.RawMessage(`{"transaction_id":1000000009,"created_timestamp":100}`)
	linked := json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":500}`)
	for _, r := range []json.RawMessage{review, linked} {
		if e := s.UpsertReviews(r); e != nil {
			t.Fatal(e)
		}
	}
	receipt := json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true,"transactions":[{"transaction_id":1000000003}]}`)
	if e := s.UpsertReceipts(receipt); e != nil {
		t.Fatal(e)
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM etsy_retention_floor WHERE scope='reviews'`) != 0 {
		t.Fatal("receipts set reviews floor")
	}
	for i := 0; i < 2; i++ {
		for _, r := range []json.RawMessage{review, linked} {
			if e := s.UpsertReviews(r); e != nil {
				t.Fatal(e)
			}
		}
		if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000010,"created_timestamp":500,"is_shipped":true}`)); e != nil {
			t.Fatal(e)
		}
	}
	for _, q := range []string{`SELECT count(*) FROM reviews WHERE transaction_id=1000000009`, `SELECT count(*) FROM resources WHERE resource_type='reviews' AND id='1000000009'`} {
		if retentionCount(t, s, q) != 1 {
			t.Fatal("unlinked old review lost")
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM reviews WHERE transaction_id=1000000003`) != 1 {
		t.Fatal("receipt prune deleted review")
	}
	// A newer review predating receipt sync has no link to pruned transactions.
	if e := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000011,"created_timestamp":500}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000010,"created_timestamp":500,"is_shipped":true,"transactions":[{"transaction_id":1000000011}]}`)); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000011,"created_timestamp":500}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM reviews WHERE transaction_id=1000000011`) != 1 {
		t.Fatal("newer review lost")
	}
}

func TestEtsyRound2OrphanAge(t *testing.T) {
	for _, kind := range []string{"transactions", "listings_transactions", "payments", "shops_receipts_payments", "receipts_listings"} {
		t.Run(kind, func(t *testing.T) {
			s := retentionStore(t)
			row := json.RawMessage(`{"transaction_id":1000000003,"payment_id":1000000003,"listing_id":1000000003,"receipt_id":1000000001,"created_timestamp":100,"shipped_timestamp":101}`)
			if _, _, e := s.UpsertBatch(kind, []json.RawMessage{row}); e != nil {
				t.Fatal(e)
			}
			result, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true})
			if e != nil {
				t.Fatal(e)
			}
			expected := 1
			if kind == "receipts_listings" {
				expected = 0
			}
			if result.Counts[kind] != expected {
				t.Fatalf("counts %+v", result.Counts)
			}
			if _, _, e := s.UpsertBatch(kind, []json.RawMessage{row}); e != nil {
				t.Fatal(e)
			}
			want := 0
			if kind == "receipts_listings" {
				want = 1
			}
			if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type=?`, kind) != want {
				t.Fatal("orphan write policy")
			}
		})
	}
}

func TestEtsyRound2ReceiptSuppression(t *testing.T) {
	s := retentionStore(t)
	row := json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":0,"buyer_email":"synthetic@example.invalid","created_timestamp":100}`)
	if e := s.UpsertReceipts(row); e != nil {
		t.Fatal(e)
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{Emails: []string{"synthetic@example.invalid"}}}); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReceipts(row); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.UpsertBatch("receipts", []json.RawMessage{row}); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`)+retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='receipts'`) != 0 {
		t.Fatal("erased receipt reimported")
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{UnsuppressReceipts: []string{"1000000001"}}}); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReceipts(row); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 1 {
		t.Fatal("receipt unsuppress failed")
	}
}

func TestEtsyRound2ParentScale(t *testing.T) {
	s := retentionStore(t)
	var receipts, transactions []json.RawMessage
	for i := 0; i < 2000; i++ {
		receipts = append(receipts, json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":500,"is_shipped":true}`, 1000000000+i)))
		transactions = append(transactions, json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d,"created_timestamp":100}`, 1000010000+i, 1000000000+i)))
	}
	if _, _, e := s.UpsertBatch("receipts", receipts); e != nil {
		t.Fatal(e)
	}
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	kept, n, e := s.FilterEtsyRetentionIncoming("transactions", transactions)
	if e != nil || n != 0 {
		t.Fatalf("%d %v", n, e)
	}
	if _, _, e = s.UpsertBatch("transactions", kept); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM transactions`) != 2000 {
		t.Fatal("write guard lost retained transactions")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("quadratic parent lookup: %v", elapsed)
	}
	if retentionCount(t, s, `SELECT count(*) FROM etsy_retention_receipts`) != 0 {
		t.Fatal("stored parents were marked dropped")
	}
}

func TestEtsyRound2BusyOptimize(t *testing.T) {
	s := retentionStore(t)
	conn, e := s.db.Conn(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	if _, e = conn.ExecContext(context.Background(), `PRAGMA busy_timeout=10`); e != nil {
		t.Fatal(e)
	}
	writer, e := s.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Rollback()
	if _, e = writer.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('listings','synthetic','{}')`); e != nil {
		t.Fatal(e)
	}
	steps := map[string]string{}
	start := time.Now()
	status, e := etsyVacuum(context.Background(), conn, steps)
	if time.Since(start) < 10*time.Millisecond {
		t.Fatal("FTS skipped the prior busy timeout")
	}
	if e != nil || steps["optimize"] != "skipped_busy" || status == "completed" {
		t.Fatalf("%s %+v %v", status, steps, e)
	}
	if e = writer.Rollback(); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"resources_fts", "transactions_fts"} {
		if _, e = conn.ExecContext(context.Background(), `DROP TABLE "`+table+`"`); e != nil {
			t.Fatal(e)
		}
	}
	steps = map[string]string{}
	status, e = etsyVacuum(context.Background(), conn, steps)
	if e != nil || steps["optimize"] != "not_applicable" {
		t.Fatalf("no FTS: %s %+v %v", status, steps, e)
	}
}

// etsy-ext: evaluate old stored parents without marking them removed; twins share bare ids.
func TestEtsyRound2ParentDecisionAndTwins(t *testing.T) {
	s := retentionStore(t)
	if _, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Cutoff: 200}); e != nil {
		t.Fatal(e)
	}
	retentionExec(t, s, `INSERT INTO receipts(id,receipt_id,created_timestamp,is_shipped,data) VALUES('1000000001',1000000001,100,1,'{}')`)
	if e := s.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM etsy_retention_receipts`) != 0 {
		t.Fatal("parent evaluation marked a stored receipt")
	}
	for _, pair := range [][2]string{{"transactions", "listings_transactions"}, {"payments", "shops_receipts_payments"}} {
		field := "transaction_id"
		if pair[0] == "payments" {
			field = "payment_id"
		}
		for _, kind := range pair {
			retentionExec(t, s, `INSERT INTO resources(resource_type,id,data) VALUES(?,?,?)`, kind, "1000000040\x001000000001", fmt.Sprintf(`{"%s":1000000040,"receipt_id":1000000001}`, field))
		}
		if e := s.Upsert(pair[0], "1000000040", json.RawMessage(fmt.Sprintf(`{"%s":1000000040,"receipt_id":1000000001}`, field))); e != nil {
			t.Fatal(e)
		}
		for _, kind := range pair {
			if retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type=? AND id=?`, kind, "1000000040\x001000000001") != 1 {
				t.Fatalf("stored twin %s lost", kind)
			}
		}
	}
}
