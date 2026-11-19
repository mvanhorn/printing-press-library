package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEtsyErasureSelectionAndSuppression(t *testing.T) {
	s := retentionStore(t)
	receipt := json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"buyer_email":" canary-buyer@example.invalid ","created_timestamp":100,"is_shipped":false}`)
	if err := s.UpsertReceipts(receipt); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001,"buyer_user_id":1000000002,"created_timestamp":100}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100}`)); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"listings_transactions", "shops_receipts_payments", "receipts_listings"} {
		if err := s.Upsert(kind, "pp:params:"+kind, json.RawMessage(`{"receipt_id":"1000000001","buyer_user_id":"1000000002","created_timestamp":100}`)); err != nil {
			t.Fatal(err)
		}
	}
	retentionExec(t, s, `INSERT INTO user_addresses(id,data) VALUES('1000000004','{}')`)
	o := EtsyPruneOptions{Receipts: true, Erase: &EtsyEraseOptions{Emails: []string{"CANARY-BUYER@example.invalid"}, ErasedAt: 200}}
	preview, err := s.EtsyPrune(o)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Counts["reviews"] != 1 || preview.Counts["transactions"] != 1 {
		t.Fatal(preview)
	}
	o.Apply = true
	o.Vacuum = true
	result, err := s.EtsyPrune(o)
	if err != nil {
		t.Fatal(err)
	}
	if result.Vacuum != "completed" {
		t.Fatal(result)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 0 || retentionCount(t, s, `SELECT count(*) FROM reviews`) != 0 {
		t.Fatal("cascade left data")
	}
	if retentionCount(t, s, `SELECT count(*) FROM user_addresses`) != 1 {
		t.Fatal("seller addresses deleted")
	}
	for _, write := range []func() error{
		func() error {
			return s.Upsert("reviews", "1000000003", json.RawMessage(`{"transaction_id":1000000003,"buyer_user_id":1000000002,"created_timestamp":100}`))
		},
		func() error { return s.UpsertReceipts(receipt) },
		func() error {
			return s.UpsertReviews(json.RawMessage(`{"id":1000000003,"transaction_id":1000000003,"buyer_user_id":1000000002,"created_timestamp":100}`))
		},
		func() error {
			_, _, _, e := s.UpsertBatchDetailed("listings_reviews", []json.RawMessage{json.RawMessage(`{"id":1000000003,"transaction_id":1000000003,"buyer_user_id":1000000002,"created_timestamp":100}`)})
			return e
		},
	} {
		if err = write(); err != nil {
			t.Fatal(err)
		}
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 0 {
		t.Fatal("suppression bypassed")
	}
	if err = s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000005,"buyer_user_id":1000000002,"created_timestamp":201}`)); err != nil {
		t.Fatal(err)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 1 {
		t.Fatal("new receipt suppressed")
	}
	_, err = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{Unsuppress: []string{"1000000002"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpsertReceipts(receipt); err != nil {
		t.Fatal(err)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 2 {
		t.Fatal("unsuppress failed")
	}
}

func TestEtsyErasureLegacyAbsentTablesAndOwner(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, `INSERT INTO receipts(id,data,receipt_id,buyer_user_id,buyer_email) VALUES('1000000001','{}',1000000001,1000000002,'canary-buyer@example.invalid')`)
	for _, table := range []string{"search_learnings", "learn_recall_misses", "search_patterns", "learn_candidates", "learning_playbooks"} {
		retentionExec(t, s, `DROP TABLE IF EXISTS `+table)
	}
	retentionExec(t, s, `UPDATE receipts SET seller_user_id=1000000002`)
	_, err := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{Emails: []string{"canary-buyer@example.invalid"}}})
	if err == nil {
		t.Fatal("owner erased")
	}
	retentionExec(t, s, `UPDATE receipts SET seller_user_id=NULL`)
	_, err = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{Emails: []string{"canary-buyer@example.invalid"}}})
	if err != nil {
		t.Fatal(err)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 0 {
		t.Fatal("typed-only receipt left")
	}
	for _, id := range []string{"0", "-1", "invalid"} {
		_, err = s.EtsyPrune(EtsyPruneOptions{Receipts: true, Erase: &EtsyEraseOptions{BuyerIDs: []string{id}}})
		if err == nil {
			t.Fatal("invalid id allowed")
		}
	}
}

func TestEtsyErasureBusyVacuum(t *testing.T) {
	s := retentionStore(t)
	if err := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"created_timestamp":100}`)); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnlyContext(context.Background(), s.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	conn, err := reader.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	read, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback()
	var n int
	if err = read.QueryRow(`SELECT count(*) FROM receipts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	result, err := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Vacuum: true, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Vacuum != "checkpoint_busy" || retentionCount(t, s, `SELECT count(*) FROM receipts`) != 0 {
		t.Fatal(result)
	}
}

func TestEtsyErasureEmbeddedTransactions(t *testing.T) {
	for _, erase := range []bool{false, true} {
		t.Run(fmt.Sprint(erase), func(t *testing.T) {
			s := retentionStore(t)
			if err := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"created_timestamp":100,"is_shipped":true,"transactions":[{"transaction_id":1000000003}]}`)); err != nil {
				t.Fatal(err)
			}
			retentionExec(t, s, `INSERT INTO reviews(id,data,transaction_id) VALUES('pp:params:synthetic','{}',1000000003)`)
			o := EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true}
			if erase {
				o.Erase = &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}
			}
			result, err := s.EtsyPrune(o)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if erase {
				want = 1
			}
			if result.Counts["reviews"] != want || retentionCount(t, s, `SELECT count(*) FROM reviews`) != 1-want {
				t.Fatal("embedded transaction link lost", result)
			}
		})
	}
}

func TestEtsyErasureIndependentBuyers(t *testing.T) {
	for _, selector := range []string{"buyer", "email", "receipt"} {
		t.Run(selector, func(t *testing.T) {
			s := retentionStore(t)
			for i := int64(0); i < 2; i++ {
				receipt, buyer, transaction := 1000000001+i*10, 1000000002+i*10, 1000000003+i*10
				email := []string{"canary-buyer@example.invalid", "other-buyer@example.invalid"}[i]
				if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":%d,"buyer_user_id":%d,"buyer_email":%q,"created_timestamp":100}`, receipt, buyer, email))); e != nil {
					t.Fatal(e)
				}
				if e := s.UpsertTransactions(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"receipt_id":%d,"buyer_user_id":%d,"title":%q}`, transaction, receipt, buyer, email))); e != nil {
					t.Fatal(e)
				}
				if e := s.UpsertReviews(json.RawMessage(fmt.Sprintf(`{"transaction_id":%d,"review":%q}`, transaction, email))); e != nil {
					t.Fatal(e)
				}
			}
			o := &EtsyEraseOptions{ErasedAt: 200}
			switch selector {
			case "buyer":
				o.BuyerIDs = []string{"1000000002"}
			case "email":
				o.Emails = []string{"canary-buyer@example.invalid"}
			case "receipt":
				o.ReceiptIDs = []string{"1000000001"}
			}
			r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: o})
			if e != nil {
				t.Fatal(e)
			}
			for _, kind := range []string{"receipts", "transactions", "reviews"} {
				if r.Counts[kind] != 1 {
					t.Fatalf("target counts: %+v", r)
				}
				if retentionCount(t, s, `SELECT count(*) FROM "`+kind+`"`) != 1 {
					t.Fatalf("other buyer lost in %s", kind)
				}
			}
			if retentionCount(t, s, `SELECT count(*) FROM receipts WHERE receipt_id=1000000011`) != 1 || retentionCount(t, s, `SELECT count(*) FROM transactions WHERE transaction_id=1000000013`) != 1 || retentionCount(t, s, `SELECT count(*) FROM reviews WHERE transaction_id=1000000013`) != 1 {
				t.Fatal("wrong survivor")
			}
			if retentionCount(t, s, `SELECT count(*) FROM resources_fts WHERE rowid=?`, ftsRowID("transactions", "1000000013")) != 1 {
				t.Fatal("other FTS lost")
			}
			if retentionCount(t, s, `SELECT count(*) FROM etsy_erasure_suppression`) != 1 || retentionCount(t, s, `SELECT count(*) FROM etsy_erasure_suppression WHERE buyer_user_id='1000000002'`) != 1 {
				t.Fatal("wrong suppression")
			}
		})
	}
}

func TestEtsySuppressionMissingTimestamp(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, etsySuppressionSchema)
	retentionExec(t, s, `INSERT INTO etsy_erasure_suppression VALUES('1000000002',200)`)
	user := json.RawMessage(`{"user_id":1000000002}`)
	if e := s.UpsertUsers(user); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.UpsertBatch("users", []json.RawMessage{user}); e != nil {
		t.Fatal(e)
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM resources`) != 0 || retentionCount(t, s, `SELECT count(*) FROM users`) != 0 {
		t.Fatal("timestamp-less reimport")
	}
	if e := s.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"created_timestamp":201}`)); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM receipts`) != 1 {
		t.Fatal("new receipt lost")
	}
	// Simulate an older binary retaining a stale projection under suppression.
	retentionExec(t, s, `INSERT INTO users(id,data,user_id) VALUES('1000000002','{"user_id":1000000002}',1000000002)`)
	retentionExec(t, s, `INSERT INTO resources(resource_type,id,data) VALUES('users','1000000002','{"user_id":1000000002}')`)
	if e := s.UpsertUsers(user); e != nil {
		t.Fatal(e)
	}
	if retentionCount(t, s, `SELECT count(*) FROM users`) != 1 || retentionCount(t, s, `SELECT count(*) FROM resources WHERE resource_type='users'`) != 1 {
		t.Fatal("suppression deleted stored copies")
	}
}

func TestEtsyEraseBaseSchema(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, `DROP TABLE etsy_meta`)
	retentionExec(t, s, `INSERT INTO receipts(id,data,receipt_id,buyer_user_id) VALUES('1000000001','{}',1000000001,1000000002)`)
	for _, apply := range []bool{false, true} {
		r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: apply, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}})
		if e != nil || r.Counts["receipts"] != 1 {
			t.Fatalf("apply=%t result=%+v error=%v", apply, r, e)
		}
	}
}

func TestEtsyErasureVacuumResidueAndFTSGaps(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, `PRAGMA secure_delete=OFF`)
	canary := "zqxwvkjhg"
	// Updating a large value leaves old content on freed pages before erasure.
	if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"buyer_email":%q}`, strings.Repeat(canary, 2000)))); e != nil {
		t.Fatal(e)
	}
	retentionExec(t, s, `UPDATE receipts SET buyer_email=NULL,data='{"receipt_id":1000000001,"buyer_user_id":1000000002}'`)
	retentionExec(t, s, `UPDATE resources SET data='{"receipt_id":1000000001,"buyer_user_id":1000000002}' WHERE resource_type='receipts'`)
	if e := s.UpsertTransactions(json.RawMessage(fmt.Sprintf(`{"transaction_id":1000000003,"receipt_id":1000000001,"title":%q}`, canary))); e != nil {
		t.Fatal(e)
	}
	for _, id := range []int64{1000000011, 1000000012, 1000000013} {
		if e := s.UpsertListings(json.RawMessage(fmt.Sprintf(`{"listing_id":%d,"title":"synthetic survivor linen"}`, id))); e != nil {
			t.Fatal(e)
		}
	}
	retentionExec(t, s, `DELETE FROM listings WHERE listing_id=1000000012`)
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Vacuum: true, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}})
	if e != nil || r.Vacuum != "completed" {
		t.Fatalf("%+v %v", r, e)
	}
	files, e := filepath.Glob(s.path + "*")
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "-shm") {
			continue
		}
		body, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i+3 <= len(canary); i++ {
			if bytes.Contains(body, []byte(canary[i:i+3])) {
				t.Fatalf("trigram %q remains in %s", canary[i:i+3], file)
			}
		}
	}
	rows, e := s.db.Query(`SELECT id FROM listings WHERE rowid IN (SELECT rowid FROM listings_fts WHERE listings_fts MATCH 'linen') ORDER BY id`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	if strings.Join(ids, ",") != "1000000011,1000000013" {
		t.Fatalf("FTS survivors after rowid gap: %v", ids)
	}

}

func TestEtsyErasureBusySecureDelete(t *testing.T) {
	s := retentionStore(t)
	canary := "canary-buyer@example.invalid"
	if e := s.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"buyer_email":%q}`, canary))); e != nil {
		t.Fatal(e)
	}
	reader, e := OpenReadOnlyContext(context.Background(), s.path)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	tx, e := reader.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	var n int
	if e = tx.QueryRow(`SELECT count(*) FROM receipts`).Scan(&n); e != nil {
		t.Fatal(e)
	}
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Vacuum: false, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}})
	if e != nil {
		t.Fatal(e)
	}
	if r.Vacuum != "" {
		t.Fatal(r)
	}
	// A WAL reader does not necessarily block VACUUM. Isolate secure_delete
	// by opting out of reclamation, then checkpoint only after the reader exits.
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	retentionExec(t, s, `PRAGMA wal_checkpoint(TRUNCATE)`)
	body, e := os.ReadFile(s.path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(body, []byte(canary)) {
		t.Fatal("main db retains deleted canary")
	}
}

func TestEtsyErasureShopOwnerAndEmailWithoutBuyer(t *testing.T) {
	s := retentionStore(t)
	retentionExec(t, s, `INSERT INTO shops(id,data,shop_id,user_id) VALUES('1000000001','{}',1000000001,1000000002)`)
	_, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Erase: &EtsyEraseOptions{ShopID: "1000000001", BuyerIDs: []string{"1000000002"}}})
	if e == nil {
		t.Fatal("shop owner erased")
	}
	retentionExec(t, s, `INSERT INTO receipts(id,data,receipt_id,buyer_user_id,buyer_email) VALUES('1000000003','{}',1000000003,0,'canary-buyer@example.invalid')`)
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Apply: true, Erase: &EtsyEraseOptions{Emails: []string{"canary-buyer@example.invalid"}}})
	if e != nil || r.Counts["receipts"] != 1 {
		t.Fatalf("email with missing buyer: %+v %v", r, e)
	}
}

func TestEtsyErasureReportsUnlinkableReviews(t *testing.T) {
	s := retentionStore(t)
	if e := s.UpsertReviews(json.RawMessage(`{"transaction_id":1000000003,"review":"synthetic orphan"}`)); e != nil {
		t.Fatal(e)
	}
	r, e := s.EtsyPrune(EtsyPruneOptions{Receipts: true, Erase: &EtsyEraseOptions{BuyerIDs: []string{"1000000002"}}})
	if e != nil || r.KeptUnlinkableReviews != 1 {
		t.Fatalf("%+v %v", r, e)
	}
}
