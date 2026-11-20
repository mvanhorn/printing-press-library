package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func pruneFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true}`)))
	s2Must(t, db.Close())
	return path
}
func pruneRun(t *testing.T, path string, extra ...string) map[string]any {
	t.Helper()
	args := []string{"local-data", "prune", "--db", path, "--older-than", "1h", "--json"}
	out, _, e := s2Run(t, append(args, extra...)...)
	s2Must(t, e)
	var result map[string]any
	s2Must(t, json.Unmarshal([]byte(out), &result))
	return result
}
func TestEtsyPruneCLIContracts(t *testing.T) {
	syncCLIIsolate(t)
	for _, key := range []string{cliutil.VerifyEnvVar, cliutil.DogfoodEnvVar, cliutil.MCPChildEnvVar} {
		t.Setenv(key, "")
	}
	path := pruneFixture(t)
	before, readErr := os.ReadFile(path)
	s2Must(t, readErr)
	preview := pruneRun(t, path)
	global := pruneRun(t, path, "--yes", "--dry-run")
	if preview["dry_run"] != true || global["dry_run"] != true || !reflect.DeepEqual(preview["counts"], global["counts"]) {
		t.Fatal(preview, global)
	}
	for _, key := range []string{cliutil.VerifyEnvVar, cliutil.DogfoodEnvVar} {
		t.Setenv(key, "1")
		got := pruneRun(t, path, "--yes")
		if got["dry_run"] != true {
			t.Fatal(got)
		}
		t.Setenv(key, "")
	}
	after, readErr := os.ReadFile(path)
	s2Must(t, readErr)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("preview changed database file")
	}
	applied := pruneRun(t, path, "--yes")
	if applied["dry_run"] != false || !reflect.DeepEqual(preview["counts"], applied["counts"]) {
		t.Fatal(preview, applied)
	}
	if strings.Contains(fmt.Sprint(applied), "1000000001") {
		t.Fatal("output leaked identifier")
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "1")
	for _, extra := range [][]string{nil, {"--yes"}, {"--dry-run"}, {"--clear-floor", "--yes"}} {
		args := []string{"local-data", "prune", "--db", path}
		args = append(args, extra...)
		_, _, e := s2Run(t, args...)
		if ExitCode(e) != 2 || !strings.Contains(e.Error(), "unavailable over MCP") {
			t.Fatalf("%v", e)
		}
	}
}

func TestEtsyPruneFloorNormalSyncAndPaymentsHistory(t *testing.T) {
	syncCLIIsolate(t)
	path := pruneFixture(t)
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.RecordPaymentCheck("1000000001", sql.NullInt64{Int64: 100, Valid: true}))
	s2Must(t, db.Close())
	pruneRun(t, path, "--receipts", "--yes")
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		if r.URL.Path == "/shops/9000001/receipts" {
			fmt.Fprint(w, `{"results":[{"receipt_id":1000000001,"created_timestamp":100,"updated_timestamp":100,"is_shipped":true}],"count":1}`)
		} else if strings.HasSuffix(r.URL.Path, "/payments") {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"error":"synthetic access denial"}`)
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
	firstOutput, firstErr := syncCLIRun(t, path)
	if firstErr != nil {
		t.Fatalf("%v: %s", firstErr, firstOutput)
	}
	db, e = store.Open(path)
	s2Must(t, e)
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	s2Must(t, db.Close())
	if n != 0 {
		t.Fatal("sync bypassed floor")
	}
	out, _, e := s2Run(t, "local-data", "prune", "--db", path, "--clear-floor", "--receipts", "--yes", "--json")
	s2Must(t, e)
	if strings.Contains(out, `"reviews":`) || strings.Contains(out, `"receipts":`) {
		t.Fatal(out)
	}
	output, e := syncCLIRun(t, path)
	s2Must(t, e)
	if !strings.Contains(output, "payments_forbidden") {
		t.Fatal("403 path not exercised", output)
	}
	db, e = store.Open(path)
	s2Must(t, e)
	defer db.Close()
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	if n != 1 {
		t.Fatal("clear did not permit normal sync")
	}
	checks, e := db.ReadPaymentChecks()
	s2Must(t, e)
	if len(checks) != 1 || !checks["1000000001"].Forbidden {
		t.Fatal(checks)
	}
}

func TestEtsyDependentSyncRetentionFiltered(t *testing.T) {
	for _, suppression := range []bool{false, true} {
		t.Run(fmt.Sprint(suppression), func(t *testing.T) {
			eraseIsolate(t)
			db, e := store.Open(filepath.Join(t.TempDir(), "data.db"))
			s2Must(t, e)
			defer db.Close()
			s2Must(t, db.UpsertListings(json.RawMessage(`{"listing_id":1000000001,"title":"synthetic listing"}`)))
			if suppression {
				_, e = db.DB().Exec(`CREATE TABLE etsy_erasure_suppression(buyer_user_id TEXT PRIMARY KEY,erased_at INTEGER); INSERT INTO etsy_erasure_suppression VALUES('1000000002',200)`)
			} else {
				_, e = db.EtsyPrune(store.EtsyPruneOptions{Reviews: true, Cutoff: 200, Apply: true})
			}
			s2Must(t, e)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 1 {
					fmt.Fprint(w, `{"results":[],"count":0}`)
					return
				}
				fmt.Fprint(w, `{"results":[{"transaction_id":1000000003,"buyer_user_id":1000000002,"created_timestamp":100,"review":"synthetic review"}],"count":1}`)
			}))
			defer srv.Close()
			c := &shopScopeSyncClient{url: srv.URL, shopID: "1000000004", client: srv.Client()}
			var events bytes.Buffer
			results := syncDependentResources(context.Background(), c, db, "", false, 0, false, false, []string{"listings_reviews"}, nil, &events, 1)
			if len(results) != 1 {
				t.Fatal(results)
			}
			for _, r := range results {
				s2Must(t, r.Err)
				if r.Warn != nil {
					t.Fatal(r.Warn)
				}
			}
			if !strings.Contains(events.String(), `"event":"retention_filtered"`) || strings.Contains(events.String(), "integrity") {
				t.Fatal(events.String())
			}
			n, e := db.Count("listings_reviews")
			s2Must(t, e)
			if n != 0 {
				t.Fatal(n)
			}
		})
	}
}

func TestEtsyPruneKeepsCurrentReceipt(t *testing.T) {
	eraseIsolate(t)
	path := pruneFixture(t)
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.UpsertReceipts(json.RawMessage(fmt.Sprintf(`{"receipt_id":1000000002,"created_timestamp":%d,"is_shipped":true}`, time.Now().Unix()))))
	s2Must(t, db.Close())
	r := pruneRun(t, path, "--yes")
	if r["counts"].(map[string]any)["receipts"] != float64(1) {
		t.Fatal(r)
	}
	db, e = store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts WHERE receipt_id=1000000002`).Scan(&n))
	if n != 1 {
		t.Fatal("current receipt erased")
	}
}

// etsy-ext: fake-server re-import rejects pruned parents and their dependents.
func TestEtsySimplifiedFakeSync(t *testing.T) {
	syncCLIIsolate(t)
	path := pruneFixture(t)
	pruneRun(t, path, "--receipts", "--yes")
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000010,"created_timestamp":100,"is_shipped":false}`)))
	// This unrelated retained old copy must survive policy checks on another id.
	s2Must(t, db.UpsertListings(json.RawMessage(`{"listing_id":1000000050,"title":"synthetic"}`)))
	s2Must(t, db.Close())
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		if r.URL.Path == "/shops/9000001/receipts" {
			fmt.Fprint(w, `{"results":[{"receipt_id":1000000001,"created_timestamp":100,"is_shipped":true},{"receipt_id":1000000020,"created_timestamp":4102444800,"is_shipped":true}],"count":2}`)
		} else {
			fmt.Fprint(w, `{"results":[],"count":0}`)
		}
		return true
	})
	out, e := syncCLIRun(t, path)
	if e != nil {
		t.Fatalf("%s %v", out, e)
	}
	db, e = store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow("SELECT count(*) FROM receipts").Scan(&n))
	if n != 2 {
		t.Fatal("new/unrelated receipt lost or pruned receipt restored", n)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			fmt.Fprint(w, `{"results":[],"count":0}`)
			return
		}
		fmt.Fprint(w, `{"results":[{"transaction_id":1000000003,"receipt_id":1000000001,"created_timestamp":4102444800}],"count":1}`)
	}))
	defer srv.Close()
	c := &shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}
	var events bytes.Buffer
	for _, r := range syncDependentResources(context.Background(), c, db, "", false, 0, false, false, []string{"listings_transactions"}, nil, &events, 1) {
		s2Must(t, r.Err)
	}
	n, e = db.Count("listings_transactions")
	s2Must(t, e)
	if n != 0 {
		t.Fatal("dependent reimported", n)
	}
}

func TestEtsySimplifiedUnsuppressCLI(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	_, e := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes", "--no-vacuum")
	s2Must(t, e)
	for _, args := range [][]string{{"--unsuppress", "1000000002", "--receipt-id", "1000000001"}, {"--unsuppress=1000000002", "--receipt-id", "1000000001"}} {
		_, e = eraseRun(t, "", append([]string{"--db", path}, args...)...)
		if ExitCode(e) != 2 {
			t.Fatal("combined selectors accepted", e)
		}
	}
	_, e = eraseRun(t, "", "--db", path, "--unsuppress", "1000000002", "--yes")
	s2Must(t, e)
	db, e := store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow("SELECT count(*) FROM etsy_erasure_suppression WHERE buyer_user_id='1000000002'").Scan(&n))
	if n != 0 {
		t.Fatal("space-separated value not consumed")
	}
	out, e := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--vacuum", "--yes", "--json")
	s2Must(t, e)
	if !strings.Contains(out, `"vacuum": "completed"`) {
		t.Fatal("explicit empty vacuum skipped", out)
	}
}
