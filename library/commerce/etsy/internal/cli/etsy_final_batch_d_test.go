package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsy-ext: final privacy consequences must be visible in preview and apply.
func TestEtsyFinalTransparencyCLI(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)))
	s2Must(t, db.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001}`)))
	s2Must(t, db.UpsertReviews(json.RawMessage(`{"transaction_id":1000000003,"created_timestamp":100}`)))
	s2Must(t, db.Close())
	for _, apply := range []bool{false, true} {
		args := []string{"local-data", "prune", "--db", path, "--receipts", "--older-than", "1h", "--json"}
		if apply {
			args = append(args, "--yes")
		}
		out, stderr, e := s2Run(t, args...)
		s2Must(t, e)
		if !strings.Contains(out, `"reviews_unlinkable_after_prune": 1`) || !strings.Contains(stderr, "1 reviews will no longer be erasable by buyer; add --reviews to prune them too") {
			t.Fatalf("missing warning: %s %s", out, stderr)
		}
	}
	for _, apply := range []bool{false, true} {
		args := []string{"local-data", "prune", "--db", path, "--receipts", "--clear-floor", "--json"}
		if apply {
			args = append(args, "--yes")
		}
		out, _, e := s2Run(t, args...)
		s2Must(t, e)
		if !strings.Contains(out, `"readmitted_receipts": 1`) || !strings.Contains(out, "re-admits previously pruned receipts on the next sync") {
			t.Fatal(out)
		}
	}
}

func TestEtsyFinalBuyerReceiptUnsuppressCLI(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	_, e := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes", "--no-vacuum")
	s2Must(t, e)
	for _, apply := range []bool{false, true} {
		args := []string{"--db", path, "--unsuppress", "--receipt-id", "1000000001", "--json"}
		if apply {
			args = append(args, "--yes")
		}
		out, e := eraseRun(t, "", args...)
		s2Must(t, e)
		var result map[string]any
		s2Must(t, json.Unmarshal([]byte(out), &result))
		if result["message"] != "receipt belongs to a suppressed buyer; use --unsuppress <buyer>" {
			t.Fatal(out)
		}
	}
	db, e := store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM etsy_erasure_suppression WHERE buyer_user_id='1000000002'`).Scan(&n))
	if n != 1 {
		t.Fatal("buyer unsuppressed")
	}
}

func TestEtsyFinalFloorSyncRemembersReceipt(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, e := store.Open(path)
	s2Must(t, e)
	_, e = db.EtsyPrune(store.EtsyPruneOptions{Receipts: true, Cutoff: 200, Apply: true})
	s2Must(t, e)
	s2Must(t, db.Close())
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		if r.URL.Path == "/shops/9000001/receipts" {
			fmt.Fprint(w, `{"results":[{"receipt_id":1000000001,"created_timestamp":100,"updated_timestamp":100,"is_shipped":true}],"count":1}`)
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
	out, e := syncCLIRun(t, path)
	if e != nil {
		t.Fatalf("%v: %s", e, out)
	}
	db, e = store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM etsy_retention_receipts WHERE id='1000000001'`).Scan(&n))
	if n != 1 {
		t.Fatal("sync discarded receipt before authoritative guard")
	}
}
