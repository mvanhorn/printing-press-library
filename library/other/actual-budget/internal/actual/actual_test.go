// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
)

func fixtureLedger(t *testing.T) *actual.Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	if err := actualtest.Build(path); err != nil {
		t.Fatal(err)
	}
	db, err := actual.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return actual.NewLedger(db)
}

func TestTxnsResolvesMappingsAndSkipsTombstones(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	txns, err := l.Txns(ctx, actual.TxnFilter{From: 20260701, To: 20260731})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]actual.Txn{}
	for _, tx := range txns {
		byID[tx.ID] = tx
	}
	// Merged payee KROGER #123 resolves to Kroger; deleted category maps to Groceries.
	kr3 := byID["txn-kr-3"]
	if kr3.Payee != "Kroger" || kr3.Category != "Groceries" || kr3.Date != "2026-07-19" {
		t.Fatalf("txn-kr-3 not resolved: %+v", kr3)
	}
	if _, ok := byID["txn-start-chk"]; ok {
		t.Fatal("June starting balance leaked into July range")
	}
	all, _ := l.Txns(ctx, actual.TxnFilter{})
	for _, tx := range all {
		if tx.ID == "txn-deleted" {
			t.Fatal("tombstoned transaction returned")
		}
	}
}

func TestUncategorizedExcludesTransfersParentsAndStartingBalances(t *testing.T) {
	l := fixtureLedger(t)
	txns, err := l.Txns(context.Background(), actual.TxnFilter{Uncategorized: true})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tx := range txns {
		got[tx.ID] = true
	}
	for _, want := range []string{"txn-kr-6", "txn-ch-2", "txn-am-2", "txn-dup-a", "txn-dup-b"} {
		if !got[want] {
			t.Errorf("expected %s in uncategorized", want)
		}
	}
	for _, not := range []string{"txn-xfer-out", "txn-split", "txn-start-sav", "txn-start-brk", "txn-cardpay-in"} {
		if got[not] {
			t.Errorf("%s must not be uncategorized", not)
		}
	}
}

func TestAccountsBalancesUseLeavesOnly(t *testing.T) {
	l := fixtureLedger(t)
	accts, err := l.Accounts(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var card *actual.Account
	for i := range accts {
		if accts[i].ID == actualtest.AcctCard {
			card = &accts[i]
		}
		if accts[i].ID == "acct-gone" {
			t.Fatal("tombstoned account returned")
		}
	}
	if card == nil {
		t.Fatal("card account missing")
	}
	// Split parent (-6000) must not be double counted with its children (-2000,-4000).
	want := int64(-8450 - 9120 - 7600 - 10230 - 1500 - 8800 - 1845 - 2210 - 3999 - 2599 - 1250 - 4210 - 4210 - 1549 - 2000 - 4000 + 40000)
	if card.Balance != want {
		t.Fatalf("card balance = %d, want %d", card.Balance, want)
	}
}

func TestPayeesCountsMergedUsage(t *testing.T) {
	l := fixtureLedger(t)
	payees, err := l.Payees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range payees {
		if p.ID == actualtest.PayeeKroger && p.TxnCount != 6 {
			t.Fatalf("Kroger count = %d, want 6 (incl. merged KROGER #123)", p.TxnCount)
		}
		if p.TransferAcct != "" {
			t.Fatalf("transfer payee %s returned", p.ID)
		}
	}
}

func TestAmountAndDateHelpers(t *testing.T) {
	cases := map[string]int64{"120.30": 12030, "-5": -500, "1,234.5": 123450, "(12.00)": -1200, "$7.05": 705}
	for in, want := range cases {
		got, err := actual.ParseAmount(in)
		if err != nil || got != want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if actual.FormatAmount(-12030) != "-120.30" || actual.FormatAmount(5) != "0.05" {
		t.Error("FormatAmount wrong")
	}
	from, to, ym, err := actual.MonthRange("2026-02")
	if err != nil || from != 20260201 || to != 20260228 || ym != 202602 {
		t.Errorf("MonthRange = %d %d %d %v", from, to, ym, err)
	}
	if d, err := actual.ParseDate("2026-09-30"); err != nil || d != 20260930 {
		t.Errorf("ParseDate = %d %v", d, err)
	}
	if _, err := actual.ParseDate("2026-13-01"); err == nil {
		t.Error("ParseDate accepted month 13")
	}
}

func TestParseAmountEdgeCases(t *testing.T) {
	ok := []struct {
		in   string
		want int64
	}{
		{"-$5.00", -500},
		{"$-5", -500},
		{"+$5", 500},
		{"(12.34)", -1234},
		{"($12.34)", -1234},
		{"1,234.56", 123456},
		{"12,345,678", 1234567800},
		{".5", 50},
		{"5.", 500},
		{" 0.07 ", 7},
	}
	for _, tc := range ok {
		got, err := actual.ParseAmount(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
	bad := []struct{ in, why string }{
		{"12.345", "more than 2 decimal places"},
		{"5.-3", "want digits"},
		{"1.234,56", "European"},
		{"12,50", "thousands separator"},
		{"--5", "want digits"},
		{"-$-5", "want digits"},
		{"$", "empty"},
		{".", "no digits"},
		{"abc", "want digits"},
		{"99999999999999999999", "out of range"},
	}
	for _, tc := range bad {
		got, err := actual.ParseAmount(tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.why) {
			t.Errorf("ParseAmount(%q) = %d, %v; want error containing %q", tc.in, got, err, tc.why)
		}
	}
}

// mutatedFixtureLedger builds the fixture, applies stmts on a writable
// connection, then reopens it read-only like the real mirror.
func mutatedFixtureLedger(t *testing.T, stmts ...string) *actual.Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	if err := actualtest.Build(path); err != nil {
		t.Fatal(err)
	}
	w, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range stmts {
		if _, err := w.Exec(s); err != nil {
			w.Close()
			t.Fatalf("%s: %v", s, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := actual.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return actual.NewLedger(db)
}

func TestHasColumnDoesNotCacheErrors(t *testing.T) {
	l := fixtureLedger(t)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if l.HasColumn(cancelled, "transactions", "starting_balance_flag") {
		t.Fatal("HasColumn with a cancelled context reported true")
	}
	if !l.HasColumn(context.Background(), "transactions", "starting_balance_flag") {
		t.Fatal("failed lookup was cached: column reported missing after a good query")
	}
}

func TestSchemaDriftDroppedColumns(t *testing.T) {
	l := mutatedFixtureLedger(t,
		"ALTER TABLE transactions DROP COLUMN starting_balance_flag",
		"ALTER TABLE transactions DROP COLUMN schedule",
		"ALTER TABLE categories DROP COLUMN goal_def")
	ctx := context.Background()
	unc, err := l.Txns(ctx, actual.TxnFilter{Uncategorized: true, From: 20260701})
	if err != nil {
		t.Fatalf("uncategorized without starting_balance_flag: %v", err)
	}
	got := map[string]bool{}
	for _, tx := range unc {
		got[tx.ID] = true
	}
	for _, want := range []string{"txn-kr-6", "txn-ch-2", "txn-am-2"} {
		if !got[want] {
			t.Errorf("%s missing from uncategorized: %v", want, got)
		}
	}
	cf, err := l.Cashflow(ctx, actual.ReportScope{From: 20260801, To: 20260831})
	if err != nil {
		t.Fatalf("cashflow: %v", err)
	}
	if len(cf) != 1 || cf[0].Income != 400000 || cf[0].Spending != 173723 {
		t.Fatalf("cashflow = %+v", cf)
	}
	bva, err := l.BudgetVsActual(ctx, 202609, "")
	if err != nil {
		t.Fatalf("budget vs actual: %v", err)
	}
	if bva.TotalBudgeted != 220000 || bva.TotalSpent != 166250 {
		t.Fatalf("budget vs actual totals = %d/%d", bva.TotalBudgeted, bva.TotalSpent)
	}
	if _, err := l.Categories(ctx); err != nil {
		t.Fatalf("categories without goal_def: %v", err)
	}
}

func TestBudgetedAmountsFollowsBudgetType(t *testing.T) {
	ctx := context.Background()
	reflectRow := "INSERT INTO reflect_budgets (id,month,category,amount) VALUES ('r-groc',202609,'cat-groceries',12345)"
	prefs := "CREATE TABLE preferences (id TEXT PRIMARY KEY, value TEXT)"
	cases := []struct {
		name  string
		stmts []string
		want  int64 // groceries, Sept 2026
	}{
		{"no preference: zero_budgets has rows", []string{reflectRow}, 40000},
		{"tracking budget", []string{reflectRow, prefs, "INSERT INTO preferences VALUES ('budgetType','report')"}, 12345},
		{"envelope budget", []string{reflectRow, prefs, "INSERT INTO preferences VALUES ('budgetType','rollover')"}, 40000},
		{"unknown value falls back", []string{reflectRow, prefs, "INSERT INTO preferences VALUES ('budgetType','weird')"}, 40000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := mutatedFixtureLedger(t, tc.stmts...)
			got, err := l.BudgetedAmounts(ctx, 202609)
			if err != nil {
				t.Fatal(err)
			}
			if got["cat-groceries"] != tc.want {
				t.Fatalf("groceries budgeted = %d, want %d (all: %v)", got["cat-groceries"], tc.want, got)
			}
		})
	}
	// A tracking budget with no reflect rows for the month must not fall
	// back to stale envelope rows.
	l := mutatedFixtureLedger(t, prefs, "INSERT INTO preferences VALUES ('budgetType','report')")
	got, err := l.BudgetedAmounts(ctx, 202609)
	if err != nil || len(got) != 0 {
		t.Fatalf("tracking budget without rows = %v, %v; want empty", got, err)
	}
}

// fakeServer serves the sync-server endpoints Pull uses.
func fakeServer(t *testing.T, zipBytes []byte, meta *actual.EncryptMeta, salt string) *httptest.Server {
	t.Helper()
	ok := func(w http.ResponseWriter, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": data})
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"build":{"version":"26.9.0"}}`))
	})
	mux.HandleFunc("/account/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["password"] != "hunter2" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"status":"error","reason":"invalid-password"}`))
			return
		}
		ok(w, map[string]string{"token": "tok-1"})
	})
	authed := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-ACTUAL-TOKEN") != "tok-1" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"status":"error","reason":"unauthorized","details":"token-not-found"}`))
				return
			}
			h(w, r)
		}
	}
	keyID := ""
	if meta != nil {
		keyID = meta.KeyID
	}
	mux.HandleFunc("/sync/list-user-files", authed(func(w http.ResponseWriter, r *http.Request) {
		ok(w, []map[string]any{
			{"fileId": "file-1", "groupId": "group-1", "name": "Family Budget", "encryptKeyId": keyID, "deleted": 0},
			{"fileId": "file-old", "groupId": "group-old", "name": "Deleted", "deleted": 1},
		})
	}))
	mux.HandleFunc("/sync/get-user-file-info", authed(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-ACTUAL-FILE-ID") != "file-1" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"status":"error","reason":"file-not-found"}`))
			return
		}
		ok(w, map[string]any{"fileId": "file-1", "groupId": "group-1", "name": "Family Budget", "encryptMeta": meta})
	}))
	mux.HandleFunc("/sync/user-get-key", authed(func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]string{"id": keyID, "salt": salt, "test": "x"})
	}))
	mux.HandleFunc("/sync/download-user-file", authed(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func budgetZip(t *testing.T) []byte {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "src.sqlite")
	if err := actualtest.Build(dbPath); err != nil {
		t.Fatal(err)
	}
	dbBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"db.sqlite": dbBytes, "metadata.json": []byte(`{"budgetName":"Family Budget","budgetType":"envelope"}`)} {
		w, _ := zw.Create(name)
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPullPlainBudget(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	srv := fakeServer(t, budgetZip(t), nil, "")
	c := actual.New(srv.URL, 10*time.Second)
	res, err := actual.Pull(context.Background(), c, actual.PullOptions{Password: "hunter2", SyncID: "group-1", KeepZip: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Name != "Family Budget" || res.Manifest.ServerVersion != "26.9.0" || res.Manifest.BudgetType != "envelope" || res.ZipPath == "" {
		t.Fatalf("unexpected manifest %+v", res)
	}
	info, err := os.Stat(res.DBPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mirror db not private: %v %v", info, err)
	}
	m, err := actual.ReadManifest("group-1")
	if err != nil || m.FileID != "file-1" {
		t.Fatalf("manifest read back: %+v %v", m, err)
	}
}

func TestPullErrors(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	srv := fakeServer(t, budgetZip(t), nil, "")
	c := actual.New(srv.URL, 10*time.Second)
	_, err := actual.Pull(context.Background(), c, actual.PullOptions{Password: "wrong", SyncID: "group-1"})
	if !actual.IsAuth(err) {
		t.Fatalf("wrong password: want auth error, got %v", err)
	}
	c = actual.New(srv.URL, 10*time.Second)
	_, err = actual.Pull(context.Background(), c, actual.PullOptions{Password: "hunter2", SyncID: "group-old"})
	if !actual.IsNotFound(err) {
		t.Fatalf("deleted budget: want not-found, got %v", err)
	}
}

func TestPullEncryptedBudget(t *testing.T) {
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	plain := budgetZip(t)
	salt := "c2FsdHNhbHRzYWx0"
	key, err := actual.DeriveKey("e2e-secret", salt)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	iv := []byte("0123456789ab")
	sealed := gcm.Seal(nil, iv, plain, nil)
	ct, tag := sealed[:len(sealed)-16], sealed[len(sealed)-16:]
	meta := &actual.EncryptMeta{KeyID: "key-1", Algorithm: "aes-256-gcm", IV: base64.StdEncoding.EncodeToString(iv), AuthTag: base64.StdEncoding.EncodeToString(tag)}
	srv := fakeServer(t, ct, meta, salt)

	_, err = actual.Pull(context.Background(), actual.New(srv.URL, 10*time.Second), actual.PullOptions{Password: "hunter2", SyncID: "group-1"})
	if !errors.Is(err, actual.ErrMissingKey) {
		t.Fatalf("no encryption password: want ErrMissingKey, got %v", err)
	}
	_, err = actual.Pull(context.Background(), actual.New(srv.URL, 10*time.Second), actual.PullOptions{Password: "hunter2", SyncID: "group-1", EncryptionPassword: "nope"})
	if !errors.Is(err, actual.ErrDecrypt) {
		t.Fatalf("wrong encryption password: want ErrDecrypt, got %v", err)
	}
	res, err := actual.Pull(context.Background(), actual.New(srv.URL, 10*time.Second), actual.PullOptions{Password: "hunter2", SyncID: "group-1", EncryptionPassword: "e2e-secret"})
	if err != nil || !res.Manifest.Encrypted {
		t.Fatalf("encrypted pull: %+v %v", res, err)
	}
}

func TestExtractArchiveNestedDir(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"My-Budget/db.sqlite", "My-Budget/metadata.json"} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(name))
	}
	_ = zw.Close()
	arc, err := actual.ExtractArchive(buf.Bytes())
	if err != nil || string(arc.DB) != "My-Budget/db.sqlite" {
		t.Fatalf("nested archive: %v %v", arc, err)
	}
	if _, err := actual.ExtractArchive([]byte("not a zip")); err == nil {
		t.Fatal("garbage accepted as zip")
	}
}
