package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

const erasureCanary = "canary-buyer@example.invalid"

func eraseRun(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"local-data", "erase-buyer"}, args...))
	err := root.Execute()
	return out.String(), err
}
func eraseIsolate(t *testing.T) {
	t.Helper()
	syncCLIIsolate(t)
	for _, key := range []string{cliutil.VerifyEnvVar, cliutil.DogfoodEnvVar, cliutil.MCPChildEnvVar} {
		t.Setenv(key, "")
	}
}
func seedErase(t *testing.T, path string) {
	t.Helper()
	db, err := store.Open(path)
	s2Must(t, err)
	s2Must(t, db.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000001,"buyer_user_id":1000000002,"buyer_email":"canary-buyer@example.invalid","created_timestamp":100,"is_shipped":true}`)))
	s2Must(t, db.UpsertReceipts(json.RawMessage(`{"receipt_id":1000000011,"buyer_user_id":1000000012,"buyer_email":"other-buyer@example.invalid","created_timestamp":4102444800,"is_shipped":true}`)))
	s2Must(t, db.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000013,"receipt_id":1000000011,"buyer_user_id":1000000012,"title":"other-buyer@example.invalid"}`)))
	s2Must(t, db.UpsertReviews(json.RawMessage(`{"transaction_id":1000000013,"review":"other-buyer@example.invalid","created_timestamp":4102444800}`)))
	s2Must(t, db.UpsertTransactions(json.RawMessage(`{"transaction_id":1000000003,"receipt_id":1000000001,"buyer_user_id":1000000002,"title":"canary-buyer@example.invalid","created_timestamp":100}`)))
	for _, kind := range []string{"shops_receipts_payments", "listings_transactions"} {
		s2Must(t, db.Upsert(kind, "pp:params:"+kind, json.RawMessage(`{"receipt_id":"1000000001","buyer_user_id":"1000000002","note":"canary-buyer@example.invalid"}`)))
	}
	_, err = db.DB().Exec(`INSERT INTO search_learnings(query_pattern,resource_type,resource_id,action,source,notes) VALUES('buyer receipts','receipts','1000000001','fetch','taught',?)`, erasureCanary)
	s2Must(t, err)
	_, err = db.DB().Exec(`INSERT INTO learning_playbooks(query_family,notes_text) VALUES('buyer receipts',?)`, erasureCanary)
	s2Must(t, err)
	_, err = db.DB().Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,strategy,entity_kind,source,example_query) VALUES('buyer {entity}','{entity}','receipts','substitute','buyer','taught',?)`, erasureCanary)
	s2Must(t, err)
	_, err = db.DB().Exec(`INSERT INTO learn_candidates(class,payload,derivation_signature,query_family,created_at,updated_at,last_seen_at) VALUES('playbook_candidate',?,'synthetic-erasure','buyer receipts','synthetic','synthetic','synthetic')`, erasureCanary)
	s2Must(t, err)
	_, err = db.DB().Exec(`CREATE TABLE learn_recall_misses(entity TEXT PRIMARY KEY,miss_count INTEGER)`)
	s2Must(t, err)
	_, err = db.DB().Exec(`INSERT INTO learn_recall_misses VALUES(?,1)`, erasureCanary)
	s2Must(t, err)
	s2Must(t, db.Close())
}
func TestEtsyEraseCanaryEveryStoreAndPrivacy(t *testing.T) {
	eraseIsolate(t)
	data, err := cliutil.DataDir()
	s2Must(t, err)
	s2Must(t, os.MkdirAll(data, 0700))
	paths := []string{filepath.Join(data, "data.db"), filepath.Join(data, "data-synthetic.db")}
	for _, path := range paths {
		seedErase(t, path)
	}
	// The second credential copy lacks the selector email; the resolved buyer
	// must propagate from the first store before every copy is processed.
	second, e := store.Open(paths[1])
	s2Must(t, e)
	_, e = second.DB().Exec(`UPDATE receipts SET buyer_email=NULL,data=json_remove(data,'$.buyer_email')`)
	s2Must(t, e)
	_, e = second.DB().Exec(`UPDATE resources SET data=json_remove(data,'$.buyer_email') WHERE resource_type='receipts'`)
	s2Must(t, e)
	s2Must(t, second.Close())
	cache, err := cliutil.CacheDir()
	s2Must(t, err)
	s2Must(t, os.MkdirAll(filepath.Join(cache, "http"), 0700))
	s2Must(t, os.WriteFile(filepath.Join(cache, "http", "synthetic"), []byte(erasureCanary), 0600))
	state, err := cliutil.StateDir()
	s2Must(t, err)
	s2Must(t, os.MkdirAll(state, 0700))
	s2Must(t, os.WriteFile(filepath.Join(state, "teach.log"), []byte(`{"resource":"1000000001","detail":"canary-buyer@example.invalid"}`+"\nkeep control\n"), 0600))
	for _, jsonFlag := range []bool{false, true} {
		args := []string{"--buyer-email", "-"}
		if jsonFlag {
			args = append(args, "--json")
		}
		out, e := eraseRun(t, " CANARY-BUYER@example.invalid \n", args...)
		s2Must(t, e)
		if strings.Contains(strings.ToLower(out), erasureCanary) {
			t.Fatal("email echoed", out)
		}
	}
	out, err := eraseRun(t, erasureCanary+"\n", "--buyer-email", "-", "--yes", "--vacuum", "--json")
	s2Must(t, err)
	var result map[string]any
	s2Must(t, json.Unmarshal([]byte(out), &result))
	if result["vacuum"] != "completed" || result["http_cache_removed"] != true || len(result["stores"].([]any)) != 2 {
		t.Fatal(out)
	}
	files, err := filepath.Glob(filepath.Join(data, "data*.db*"))
	s2Must(t, err)
	for _, file := range files {
		body, e := os.ReadFile(file)
		s2Must(t, e)
		if bytes.Contains(body, []byte(erasureCanary)) {
			t.Fatalf("canary in %s", filepath.Base(file))
		}
	}
	body, err := os.ReadFile(filepath.Join(state, "teach.log"))
	s2Must(t, err)
	if bytes.Contains(body, []byte(erasureCanary)) || !bytes.Contains(body, []byte("keep control")) {
		t.Fatal("teach log rewrite", string(body))
	}
	if _, err = os.Stat(filepath.Join(cache, "http")); !os.IsNotExist(err) {
		t.Fatal("HTTP cache remains", err)
	}
}
func TestEtsyEraseCLIRefusalsAndDryRun(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	for _, args := range [][]string{{"--buyer-user-id", "0"}, {"--receipt-id", "-1"}, {"--unsuppress", "0"}} {
		_, err := eraseRun(t, "", append([]string{"--db", path}, args...)...)
		if ExitCode(err) != 2 {
			t.Fatal(args, err)
		}
	}
	t.Setenv("ETSY_OAUTH2", "1000000002.synthetic-token")
	_, err := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes")
	if ExitCode(err) != 2 {
		t.Fatal("owner", err)
	}
	t.Setenv("ETSY_OAUTH2", "")
	for _, extra := range [][]string{nil, {"--yes", "--dry-run"}} {
		out, e := eraseRun(t, erasureCanary, append([]string{"--db", path, "--buyer-email", "-", "--json"}, extra...)...)
		s2Must(t, e)
		if !strings.Contains(out, `"dry_run": true`) {
			t.Fatal(out)
		}
	}
	for _, key := range []string{cliutil.VerifyEnvVar, cliutil.DogfoodEnvVar} {
		t.Setenv(key, "1")
		out, e := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes", "--json")
		s2Must(t, e)
		if !strings.Contains(out, `"dry_run": true`) {
			t.Fatal(out)
		}
		t.Setenv(key, "")
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "1")
	for _, extra := range [][]string{{"--dry-run"}, {"--yes"}} {
		_, err = eraseRun(t, "", append([]string{"--buyer-user-id", "1000000002"}, extra...)...)
		if ExitCode(err) != 2 || !strings.Contains(err.Error(), "unavailable over MCP") {
			t.Fatal(err)
		}
	}
}
func TestEtsyEraseSuppressionNormalSync(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	_, err := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes")
	s2Must(t, err)
	future := time.Now().Add(time.Hour).Unix()
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		syncCLIQuota(w, "4000")
		if r.URL.Path == "/shops/9000001/receipts" {
			fmt.Fprintf(w, `{"results":[{"receipt_id":1000000001,"buyer_user_id":1000000002,"created_timestamp":100,"updated_timestamp":100},{"receipt_id":1000000004,"buyer_user_id":1000000002,"created_timestamp":%d,"updated_timestamp":%d}],"count":2}`, future, future)
		} else if strings.HasSuffix(r.URL.Path, "/payments") {
			fmt.Fprint(w, `{"results":[],"count":0}`)
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		return true
	})
	_, err = syncCLIRun(t, path)
	s2Must(t, err)
	db, err := store.Open(path)
	s2Must(t, err)
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	s2Must(t, db.Close())
	if n != 2 {
		t.Fatal("suppression bypassed by sync", n)
	}
	_, err = eraseRun(t, "", "--db", path, "--unsuppress", "1000000002", "--yes")
	s2Must(t, err)
	_, err = syncCLIRun(t, path)
	s2Must(t, err)
	db, err = store.Open(path)
	s2Must(t, err)
	defer db.Close()
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	if n != 3 {
		t.Fatal("unsuppress did not allow sync", n)
	}
}

func TestEtsyEraseWithoutVacuumRetainsWALCanary(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	// A read snapshot pins the pre-delete WAL frames, independent of secure_delete.
	reader, err := store.OpenReadOnlyContext(context.Background(), path)
	s2Must(t, err)
	defer reader.Close()
	tx, err := reader.DB().Begin()
	s2Must(t, err)
	defer tx.Rollback()
	var n int
	s2Must(t, tx.QueryRow(`SELECT count(*) FROM receipts`).Scan(&n))
	_, err = eraseRun(t, erasureCanary, "--db", path, "--buyer-email", "-", "--yes", "--no-vacuum")
	s2Must(t, err)
	files, err := filepath.Glob(path + "*")
	s2Must(t, err)
	found := false
	for _, file := range files {
		body, e := os.ReadFile(file)
		s2Must(t, e)
		found = found || bytes.Contains(body, []byte(erasureCanary))
	}
	if !found {
		t.Fatal("control expected pinned pre-delete bytes")
	}
}

func TestEtsyPruneCleansLearnFamiliesAndCache(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	pruneRun(t, path, "--yes", "--vacuum")
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	for _, table := range []string{"search_learnings", "search_patterns", "learning_playbooks", "learn_candidates", "learn_recall_misses"} {
		var n int
		s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM `+table).Scan(&n))
		if n != 0 {
			t.Fatal("learn data left", table)
		}
	}
}

func TestEtsyEraseCLIAbsentLearnTables(t *testing.T) {
	eraseIsolate(t)
	path := pruneFixture(t)
	db, err := store.Open(path)
	s2Must(t, err)
	for _, table := range []string{"search_learnings", "search_patterns", "learning_playbooks", "learn_candidates", "learn_recall_misses", "etsy_payment_checks"} {
		_, err = db.DB().Exec(`DROP TABLE IF EXISTS ` + table)
		s2Must(t, err)
	}
	s2Must(t, db.Close())
	_, err = eraseRun(t, "", "--db", path, "--receipt-id", "1000000001", "--yes", "--vacuum")
	s2Must(t, err)
}

func TestEtsyEraseKeepsPreScrubReviewSelection(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	s2Must(t, err)
	_, err = db.DB().Exec(`DELETE FROM etsy_meta WHERE key='review_buyer_fields_scrub_v1'`)
	s2Must(t, err)
	_, err = db.DB().Exec(`INSERT INTO reviews(id,data,buyer_user_id) VALUES('1000000003','{"buyer_user_id":1000000002,"review":"linen apron"}',1000000002)`)
	s2Must(t, err)
	s2Must(t, db.Close())
	out, err := eraseRun(t, "", "--db", path, "--buyer-user-id", "1000000002", "--yes", "--json")
	s2Must(t, err)
	var result map[string]any
	s2Must(t, json.Unmarshal([]byte(out), &result))
	if result["counts"].(map[string]any)["reviews"] != float64(1) {
		t.Fatal("scrub changed selected set", out)
	}
	db, err = store.Open(path)
	s2Must(t, err)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM reviews`).Scan(&n))
	if n != 0 {
		t.Fatal("legacy buyer review left")
	}
}

func TestEtsyEraseLogFilesAndPartialRetry(t *testing.T) {
	eraseIsolate(t)
	data, e := cliutil.DataDir()
	s2Must(t, e)
	s2Must(t, os.MkdirAll(data, 0700))
	paths := []string{filepath.Join(data, "data-a.db"), filepath.Join(data, "data-b.db")}
	for _, p := range paths {
		seedErase(t, p)
	}
	db, e := store.Open(paths[1])
	s2Must(t, e)
	_, e = db.DB().Exec(`CREATE TRIGGER synthetic_failure BEFORE DELETE ON receipts BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`)
	s2Must(t, e)
	s2Must(t, db.Close())
	state, e := cliutil.StateDir()
	s2Must(t, e)
	s2Must(t, os.MkdirAll(state, 0700))
	var logs []string
	for _, dir := range []string{state, data} {
		for _, name := range []string{"teach.log", "teach.log.1", "learnings.jsonl", "learnings.jsonl.1"} {
			p := filepath.Join(dir, name)
			logs = append(logs, p)
			s2Must(t, os.WriteFile(p, []byte("1000000002\n{\"buyer\":1000000002}\nkeep 10000000020\n"), 0600))
		}
	}
	out, e := eraseRun(t, "", "--buyer-user-id", "1000000002", "--yes", "--no-vacuum")
	if e == nil || !strings.Contains(out, "committed_stores=1") {
		t.Fatalf("partial result %v %s", e, out)
	}
	for _, p := range logs {
		body, e := os.ReadFile(p)
		s2Must(t, e)
		if string(body) != "keep 10000000020\n" {
			t.Fatalf("scrub %s: %s", p, body)
		}
	}
	db, e = store.Open(paths[1])
	s2Must(t, e)
	_, e = db.DB().Exec(`DROP TRIGGER synthetic_failure`)
	s2Must(t, e)
	s2Must(t, db.Close())
	_, e = eraseRun(t, "", "--buyer-user-id", "1000000002", "--yes")
	s2Must(t, e)
	for _, p := range paths {
		db, e := store.Open(p)
		s2Must(t, e)
		var n int
		s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM receipts WHERE buyer_user_id=1000000002`).Scan(&n))
		s2Must(t, db.Close())
		if n != 0 {
			t.Fatal("retry left target")
		}
	}
}

func TestEtsyEraseCLISelectorsKeepOtherBuyer(t *testing.T) {
	for _, selector := range [][]string{{"--buyer-user-id", "1000000002"}, {"--buyer-email", "-"}, {"--receipt-id", "1000000001"}} {
		t.Run(selector[0], func(t *testing.T) {
			eraseIsolate(t)
			path := filepath.Join(t.TempDir(), "data.db")
			seedErase(t, path)
			// Add a target review so receipt selector outcome is exercised explicitly.
			db, e := store.Open(path)
			s2Must(t, e)
			s2Must(t, db.UpsertReviews(json.RawMessage(`{"transaction_id":1000000003,"review":"synthetic target"}`)))
			s2Must(t, db.Close())
			args := append([]string{"--db", path, "--yes", "--json"}, selector...)
			out, e := eraseRun(t, erasureCanary, args...)
			s2Must(t, e)
			var result map[string]any
			s2Must(t, json.Unmarshal([]byte(out), &result))
			if result["vacuum"] != "completed" {
				t.Fatal("default vacuum missing", out)
			}
			counts := result["counts"].(map[string]any)
			for _, kind := range []string{"receipts", "transactions", "reviews"} {
				if counts[kind] != float64(1) {
					t.Fatal(out)
				}
			}
			db, e = store.Open(path)
			s2Must(t, e)
			defer db.Close()
			for _, q := range []string{`SELECT count(*) FROM receipts WHERE receipt_id=1000000011`, `SELECT count(*) FROM transactions WHERE transaction_id=1000000013`, `SELECT count(*) FROM reviews WHERE transaction_id=1000000013`, `SELECT count(*) FROM resources_fts WHERE id='1000000013' AND resource_type='transactions'`, `SELECT count(*) FROM etsy_erasure_suppression WHERE buyer_user_id='1000000002'`} {
				var n int
				s2Must(t, db.DB().QueryRow(q).Scan(&n))
				if n != 1 {
					t.Fatal("other buyer or suppression", q, n)
				}
			}
			var n int
			s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM etsy_erasure_suppression`).Scan(&n))
			if n != 1 {
				t.Fatal("other suppressed")
			}
		})
	}
}

func TestEtsyEraseFlagGuards(t *testing.T) {
	eraseIsolate(t)
	for _, tc := range []struct {
		args  []string
		stdin string
	}{
		{[]string{"--unsuppress", "1000000002", "--buyer-user-id", "1000000003"}, ""},
		{[]string{"--buyer-email", "-"}, strings.Repeat("x", 65537)},
		{[]string{"--buyer-email", "-"}, " \n"},
	} {
		_, e := eraseRun(t, tc.stdin, tc.args...)
		if ExitCode(e) != 2 {
			t.Fatalf("%v: %v", tc.args, e)
		}
	}
	for _, args := range [][]string{{"--clear-floor", "--older-than", "1h"}, {"--older-than", "0s"}, {"--older-than", "-1h"}} {
		_, _, e := s2Run(t, append([]string{"local-data", "prune"}, args...)...)
		if ExitCode(e) != 2 {
			t.Fatalf("%v: %v", args, e)
		}
	}
}

func TestEtsyEraseNoopKeepsCacheAndMisses(t *testing.T) {
	eraseIsolate(t)
	path := filepath.Join(t.TempDir(), "data.db")
	seedErase(t, path)
	cache, e := cliutil.CacheDir()
	s2Must(t, e)
	p := filepath.Join(cache, "http", "synthetic")
	s2Must(t, os.MkdirAll(filepath.Dir(p), 0700))
	s2Must(t, os.WriteFile(p, []byte("keep"), 0600))
	for _, args := range [][]string{{"--unsuppress", "1000000099"}, {"--buyer-email", "-"}} {
		_, e = eraseRun(t, "missing-buyer@example.invalid", append([]string{"--db", path, "--yes"}, args...)...)
		s2Must(t, e)
	}
	if _, e = os.Stat(p); e != nil {
		t.Fatal("no-op wiped cache", e)
	}
	db, e := store.Open(path)
	s2Must(t, e)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT count(*) FROM learn_recall_misses`).Scan(&n))
	if n != 1 {
		t.Fatal("no-op wiped misses")
	}
}

func TestEtsyPruneKeepsListingTeachLine(t *testing.T) {
	eraseIsolate(t)
	path := pruneFixture(t)
	db, e := store.Open(path)
	s2Must(t, e)
	s2Must(t, db.Upsert("receipts_listings", "1000000099\x001000000001", json.RawMessage(`{"listing_id":1000000099,"receipt_id":1000000001}`)))
	s2Must(t, db.Close())
	state, e := cliutil.StateDir()
	s2Must(t, e)
	s2Must(t, os.MkdirAll(state, 0700))
	p := filepath.Join(state, "teach.log")
	line := `{"resource_type":"listings","resource_id":"1000000099"}` + "\n"
	s2Must(t, os.WriteFile(p, []byte(line), 0600))
	pruneRun(t, path, "--yes")
	body, e := os.ReadFile(p)
	s2Must(t, e)
	if string(body) != line {
		t.Fatal("listing teaching erased")
	}
}
