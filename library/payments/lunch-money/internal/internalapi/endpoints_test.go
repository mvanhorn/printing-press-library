package internalapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PATCH: Cover captured internal endpoint paths and request bodies.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	c, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	return c, srv.Close
}

func TestTriggerPlaidFetchUsesInternalV2Path(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/v2/plaid_accounts/fetch" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "295207" {
			t.Fatalf("id query = %q", got)
		}
		if got := r.URL.Query().Get("start_date"); got != "2026-05-01" {
			t.Fatalf("start_date query = %q", got)
		}
		if got := r.URL.Query().Get("end_date"); got != "2026-05-31" {
			t.Fatalf("end_date query = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(body)) != "{}" {
			t.Fatalf("body = %s", body)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	defer done()

	status, err := c.TriggerPlaidFetch(PlaidFetchOptions{ID: 295207, StartDate: "2026-05-01", EndDate: "2026-05-31"})
	if err != nil {
		t.Fatalf("TriggerPlaidFetch: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("status = %d", status)
	}
}

func TestBudgetSetAndClearUseInternalV2Path(t *testing.T) {
	var seenPut bool
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			seenPut = true
			if r.URL.Path != "/v2/budgets" {
				t.Fatalf("put path = %s", r.URL.Path)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			if got := body["category_id"]; got != float64(1767878) {
				t.Fatalf("category_id = %#v", got)
			}
			if got := body["start_date"]; got != "2026-05-01" {
				t.Fatalf("start_date = %#v", got)
			}
			if got := body["amount"]; got != "0.01" {
				t.Fatalf("amount = %#v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case http.MethodDelete:
			if !seenPut {
				t.Fatal("delete arrived before put")
			}
			if r.URL.Path != "/v2/budgets" {
				t.Fatalf("delete path = %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("category_id"); got != "1767878" {
				t.Fatalf("delete category_id = %q", got)
			}
			if got := r.URL.Query().Get("start_date"); got != "2026-05-01" {
				t.Fatalf("delete start_date = %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	defer done()

	if _, err := c.UpsertBudget(BudgetUpsert{CategoryID: 1767878, StartDate: "2026-05-01", Amount: "0.01", Currency: "usd"}); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}
	if err := c.DeleteBudget(1767878, "2026-05-01"); err != nil {
		t.Fatalf("DeleteBudget: %v", err)
	}
}

func TestBulkInsertAndImportConfigPaths(t *testing.T) {
	var calls []string
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "PUT /transactions/bulk_insert/check":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode check body: %v", err)
			}
			if got := body["apply_rules"]; got != true {
				t.Fatalf("apply_rules = %#v", got)
			}
			if got := body["skip_duplicates"]; got != true {
				t.Fatalf("skip_duplicates = %#v", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"checked": true})
		case "PUT /transactions/bulk_insert":
			_ = json.NewEncoder(w).Encode(map[string]any{"inserted": true})
		case "GET /import_configs":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"name": "default"}})
		case "PUT /import_configs":
			_ = json.NewEncoder(w).Encode(map[string]any{"saved": true})
		default:
			t.Fatalf("unexpected call %s %s", r.Method, r.URL.Path)
		}
	})
	defer done()

	txns := []TransactionCreate{{Date: "2026-05-14", Payee: "ZZZZ_CAPTURE_DELETE_ME", Amount: -0.01, Currency: "usd"}}
	if _, err := c.BulkInsertTransactionsCheck(txns, BulkInsertOptions{ApplyRules: true, SkipDuplicates: true}); err != nil {
		t.Fatalf("BulkInsertTransactionsCheck: %v", err)
	}
	if _, err := c.BulkInsertTransactions(txns); err != nil {
		t.Fatalf("BulkInsertTransactions: %v", err)
	}
	if _, err := c.ListImportConfigs(); err != nil {
		t.Fatalf("ListImportConfigs: %v", err)
	}
	if _, err := c.SaveImportConfig(map[string]any{"name": "default"}); err != nil {
		t.Fatalf("SaveImportConfig: %v", err)
	}
	want := []string{
		"PUT /transactions/bulk_insert/check",
		"PUT /transactions/bulk_insert",
		"GET /import_configs",
		"PUT /import_configs",
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("call %d = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestMultipartUploadHelpers(t *testing.T) {
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		files := r.MultipartForm.File["file"]
		if len(files) != 1 {
			t.Fatalf("file parts = %d", len(files))
		}
		f, err := files[0].Open()
		if err != nil {
			t.Fatalf("open file part: %v", err)
		}
		data, _ := io.ReadAll(f)
		_ = f.Close()
		switch r.URL.Path {
		case "/transactions/file/123":
			if string(data) != "hello" {
				t.Fatalf("legacy file data = %q", data)
			}
			if got := files[0].Header.Get("Content-Type"); got != "text/plain" {
				t.Fatalf("legacy content-type = %q", got)
			}
		case "/transactions/file/pdf":
			if string(data) != "%PDF" {
				t.Fatalf("pdf file data = %q", data)
			}
			if got := r.MultipartForm.Value["asset_id"]; len(got) != 1 || got[0] != "456" {
				t.Fatalf("asset_id field = %#v", got)
			}
		default:
			t.Fatalf("unexpected multipart path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	defer done()

	if _, err := c.UploadTransactionFile(123, "note.txt", []byte("hello"), "text/plain"); err != nil {
		t.Fatalf("UploadTransactionFile: %v", err)
	}
	assetID := int64(456)
	if _, err := c.UploadPDF("statement.pdf", []byte("%PDF"), &assetID, nil); err != nil {
		t.Fatalf("UploadPDF: %v", err)
	}
}

// PATCH: Guard against batched rule-apply commits misrouting transactions.
func TestApplyRulesSerializesCriteriaIDs(t *testing.T) {
	var calls [][]int64
	c, done := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/rules/apply" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body struct {
			CriteriaIDs           []int64 `json:"criteria_ids"`
			DryRun                bool    `json:"dry_run"`
			IncludeTransactionIDs []int64 `json:"include_transaction_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.DryRun {
			t.Fatal("DryRun = true, want false")
		}
		if len(body.CriteriaIDs) != 1 {
			t.Fatalf("criteria_ids = %#v, want exactly one id per request", body.CriteriaIDs)
		}
		if len(body.IncludeTransactionIDs) != 2 || body.IncludeTransactionIDs[0] != 101 || body.IncludeTransactionIDs[1] != 102 {
			t.Fatalf("include_transaction_ids = %#v", body.IncludeTransactionIDs)
		}
		calls = append(calls, append([]int64(nil), body.CriteriaIDs...))
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": body.CriteriaIDs[0]}})
	})
	defer done()

	results, err := c.ApplyRules([]int64{11, 22}, []int64{101, 102})
	if err != nil {
		t.Fatalf("ApplyRules: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %#v, want two serialized calls", calls)
	}
	if calls[0][0] != 11 || calls[1][0] != 22 {
		t.Fatalf("calls = %#v, want ids in input order", calls)
	}
}
