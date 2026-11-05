// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/store"
)

// TestNovelExpensesScanDuplicatesHelpWires smoke-tests that the expenses scan-duplicates command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelExpensesScanDuplicatesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"expenses", "scan-duplicates", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expenses scan-duplicates --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "scan-duplicates"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expenses scan-duplicates --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSync_ExpensesPerReportHappyPath(t *testing.T) {
	tempHome := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "/reports") && !strings.Contains(r.URL.Path, "/expenses") {
			// List reports
			_, _ = w.Write([]byte(`{
				"links": [],
				"content": [
					{
						"reportId": "report-1",
						"name": "Report 1"
					},
					{
						"reportId": "report-2",
						"name": "Report 2"
					}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-1/expenses") {
			// Report 1 expenses
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-1",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"vendor": {"name": "Vendor A"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-2/expenses") {
			// Report 2 expenses
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-2",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"vendor": {"name": "Vendor A"},
					"transactionDate": "2026-11-02"
				}
			]`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync execution failed: %v", err)
	}

	t.Logf("stdout: %s", out.String())

	// Verify database content
	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db.Close()

	rows, err := db.DB().Query(`SELECT data FROM "expenses" ORDER BY expense_id`)
	if err != nil {
		t.Fatalf("querying expenses: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
		var data []byte
		if err := rows.Scan(&data); err != nil {
			t.Fatalf("scanning row: %v", err)
		}
		var exp map[string]any
		if err := json.Unmarshal(data, &exp); err != nil {
			t.Fatalf("unmarshaling data: %v", err)
		}

		expID := exp["expenseId"].(string)
		reportID := exp["reportId"].(string)
		reportIDUnderscore := exp["report_id"].(string)

		if expID == "exp-1" && (reportID != "report-1" || reportIDUnderscore != "report-1") {
			t.Errorf("exp-1 reportId = %q, want report-1", reportID)
		}
		if expID == "exp-2" && (reportID != "report-2" || reportIDUnderscore != "report-2") {
			t.Errorf("exp-2 reportId = %q, want report-2", reportID)
		}
	}
	if count != 2 {
		t.Errorf("got %d stored expenses, want 2", count)
	}
}

func TestSync_ExpensesPerReportPartialFailure(t *testing.T) {
	tempHome := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/reports") && !strings.Contains(r.URL.Path, "/expenses") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"links": [],
				"content": [
					{
						"reportId": "report-1",
						"name": "Report 1"
					},
					{
						"reportId": "report-2",
						"name": "Report 2"
					}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-1/expenses") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-1",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"vendor": {"name": "Vendor A"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-2/expenses") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessage": "internal error"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--json",
		"--strict", // Ensure partial failure exits non-zero under --strict
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit/error for strict partial failure, got nil")
	}

	// But exp-1 should still have been synced successfully!
	db, openErr := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if openErr != nil {
		t.Fatalf("opening test db: %v", openErr)
	}
	defer db.Close()

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM "expenses"`).Scan(&count); err != nil {
		t.Fatalf("counting stored expenses: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d stored expenses, want 1 (report-1 expenses should be committed even though report-2 failed)", count)
	}
}

func TestScanDuplicates_RefuseOnNeverSynced(t *testing.T) {
	tempHome := t.TempDir()

	// Touch empty DB
	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	db.Close()

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "scan-duplicates",
		"--home", tempHome,
	})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected scan-duplicates to fail/refuse on never-synced store, got nil")
	}
	if !strings.Contains(err.Error(), "has never been successfully synced") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestScanDuplicates_IgnoreEmptyReportIDRows(t *testing.T) {
	tempHome := t.TempDir()

	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db.Close()

	// Seed identical expenses but with empty reportId
	expBytes := []byte(`{
		"expenseId": "exp-1",
		"transactionAmount": {"value": 50.0},
		"expenseType": {"id": "01000"},
		"vendor": {"description": "Vendor A"},
		"transactionDate": "2026-11-01"
	}`)
	expBytes2 := []byte(`{
		"expenseId": "exp-2",
		"transactionAmount": {"value": 50.0},
		"expenseType": {"id": "01000"},
		"vendor": {"description": "Vendor A"},
		"transactionDate": "2026-11-01"
	}`)

	if _, _, err := db.UpsertBatch("expenses", []json.RawMessage{expBytes, expBytes2}); err != nil {
		t.Fatalf("upserting batch: %v", err)
	}

	// Seed valid sync state
	if err := db.SaveSyncStateAt("expenses", "", 2, time.Now()); err != nil {
		t.Fatalf("seeding sync state: %v", err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "scan-duplicates",
		"--home", tempHome,
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("scan-duplicates execution failed: %v", err)
	}

	var res scanDuplicatesResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		// Output might be wrapped under results
		var wrapped struct {
			Results scanDuplicatesResult `json:"results"`
		}
		if err2 := json.Unmarshal(out.Bytes(), &wrapped); err2 == nil {
			res = wrapped.Results
		} else {
			t.Fatalf("failed to parse output: %v", err)
		}
	}

	if len(res.DuplicateGroups) != 0 {
		t.Errorf("got %d duplicate groups, want 0 (empty report_id rows must be ignored)", len(res.DuplicateGroups))
	}
}

func TestScanDuplicates_DetectCrossReportDuplicate(t *testing.T) {
	tempHome := t.TempDir()

	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db.Close()

	// Seed cross-report identical expenses
	expBytes := []byte(`{
		"expenseId": "exp-1",
		"reportId": "report-1",
		"transactionAmount": {"value": 50.0},
		"expenseType": {"id": "01000"},
		"vendor": {"description": "Vendor A"},
		"transactionDate": "2026-11-01"
	}`)
	expBytes2 := []byte(`{
		"expenseId": "exp-2",
		"reportId": "report-2",
		"transactionAmount": {"value": 50.0},
		"expenseType": {"id": "01000"},
		"vendor": {"description": "Vendor A"},
		"transactionDate": "2026-11-02"
	}`)

	if _, _, err := db.UpsertBatch("expenses", []json.RawMessage{expBytes, expBytes2}); err != nil {
		t.Fatalf("upserting batch: %v", err)
	}

	// Seed valid sync state
	if err := db.SaveSyncStateAt("expenses", "", 2, time.Now()); err != nil {
		t.Fatalf("seeding sync state: %v", err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "scan-duplicates",
		"--home", tempHome,
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("scan-duplicates execution failed: %v", err)
	}

	var res scanDuplicatesResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		var wrapped struct {
			Results scanDuplicatesResult `json:"results"`
		}
		if err2 := json.Unmarshal(out.Bytes(), &wrapped); err2 == nil {
			res = wrapped.Results
		} else {
			t.Fatalf("failed to parse output: %v", err)
		}
	}

	if len(res.DuplicateGroups) != 1 {
		t.Fatalf("got %d duplicate groups, want 1", len(res.DuplicateGroups))
	}

	g := res.DuplicateGroups[0]
	if g.Vendor != "Vendor A" || g.Amount != 50.0 {
		t.Errorf("unexpected duplicate group details: %+v", g)
	}
	if len(g.Expenses) != 2 {
		t.Fatalf("got %d expenses in group, want 2", len(g.Expenses))
	}

	// Verify report_ids of members
	reports := map[string]string{
		g.Expenses[0].ExpenseId: g.Expenses[0].ReportId,
		g.Expenses[1].ExpenseId: g.Expenses[1].ReportId,
	}
	if reports["exp-1"] != "report-1" || reports["exp-2"] != "report-2" {
		t.Errorf("unexpected report ids for duplicates: %+v", reports)
	}
}

// PATCH(amend-2026-11-04: test expenses dry run does not write sync timestamp and empty-but-real reports list does)
func TestSync_ExpensesDryRunDoesNotWriteSyncTimestamp(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Dry run
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"dry_run": true}`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--dry-run",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync execution failed in dry-run: %v", err)
	}

	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}

	// Verify NO sync state row was written for expenses
	var count int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM sync_state WHERE resource_type = 'expenses'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected no sync_state row for expenses in dry-run, got %d", count)
	}
	db.Close()

	// 2. Genuinely empty reports list
	tempHome2 := t.TempDir()
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"links": [], "content": []}`))
	}))
	defer server2.Close()

	t.Setenv("CONCUR_BASE_URL", server2.URL)

	cmd2 := RootCmd()
	cmd2.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome2,
		"--json",
	})

	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetErr(os.Stderr)

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("sync execution failed with empty list: %v", err)
	}

	db2, err := store.Open(filepath.Join(tempHome2, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db2.Close()

	// Verify sync state row WAS written for expenses
	var count2 int
	_ = db2.DB().QueryRow(`SELECT COUNT(*) FROM sync_state WHERE resource_type = 'expenses' AND last_synced_at IS NOT NULL`).Scan(&count2)
	if count2 != 1 {
		t.Errorf("expected 1 sync_state row for expenses with empty list, got %d", count2)
	}
}

// PATCH(amend-2026-11-04: test deletion/pruning of deleted expenses between syncs)
func TestSync_ExpensesReconcilePruning(t *testing.T) {
	tempHome := t.TempDir()

	// First sync: Report 1 has exp-1 and exp-2. Report 2 has exp-3.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "/reports") && !strings.Contains(r.URL.Path, "/expenses") {
			_, _ = w.Write([]byte(`{
				"links": [],
				"content": [
					{"reportId": "report-1"},
					{"reportId": "report-2"}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-1/expenses") {
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-1",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"transactionDate": "2026-11-01"
				},
				{
					"expenseId": "exp-2",
					"transactionAmount": 100.0,
					"expenseType": {"id": "01000"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-2/expenses") {
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-3",
					"transactionAmount": 150.0,
					"expenseType": {"id": "01000"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}
	}))

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	server.Close()

	// Verify we have 3 expenses
	db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	var count int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM "expenses"`).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 stored expenses, got %d", count)
	}
	db.Close()

	// Second sync: Report 1 loses exp-2 (pruned). Report 2 fails to fetch (remains as is).
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/reports") && !strings.Contains(r.URL.Path, "/expenses") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"links": [],
				"content": [
					{"reportId": "report-1"},
					{"reportId": "report-2"}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-1/expenses") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-1",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-2/expenses") {
			// Failed to fetch Report 2
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorMessage": "internal error"}`))
			return
		}
	}))
	defer server2.Close()

	t.Setenv("CONCUR_BASE_URL", server2.URL)

	cmd2 := RootCmd()
	cmd2.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--full",
		"--json",
	})
	_ = cmd2.Execute() // might return error due to Report 2 failure, but Report 1 should prune!

	// Verify database state: exp-2 is gone (pruned), but exp-3 is still there (since Report 2 failed to fetch)
	db2, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db2.Close()

	rows, err := db2.DB().Query(`SELECT expense_id FROM "expenses" ORDER BY expense_id`)
	if err != nil {
		t.Fatalf("querying expenses: %v", err)
	}
	defer rows.Close()

	var remaining []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			remaining = append(remaining, id)
		}
	}

	wantRemaining := []string{"exp-1", "exp-3"}
	if len(remaining) != len(wantRemaining) || remaining[0] != wantRemaining[0] || remaining[1] != wantRemaining[1] {
		t.Errorf("expected remaining expenses %v, got %v", wantRemaining, remaining)
	}

	// Test `--no-prune` keeps exp-2 if it is synced back and lost again
	// Let's manually insert exp-2 back into db
	_, _, _ = db2.UpsertBatch("expenses", []json.RawMessage{[]byte(`{
		"expenseId": "exp-2",
		"reportId": "report-1",
		"report_id": "report-1",
		"transactionAmount": {"value": 100.0},
		"expenseType": {"id": "01000"},
		"transactionDate": "2026-11-01"
	}`)})

	// Third sync: Report 1 still only has exp-1, but we use `--no-prune` flag
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.Contains(r.URL.Path, "/reports") && !strings.Contains(r.URL.Path, "/expenses") {
			_, _ = w.Write([]byte(`{
				"links": [],
				"content": [
					{"reportId": "report-1"}
				]
			}`))
			return
		}

		if strings.Contains(r.URL.Path, "/report-1/expenses") {
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-1",
					"transactionAmount": 50.0,
					"expenseType": {"id": "01000"},
					"transactionDate": "2026-11-01"
				}
			]`))
			return
		}
	}))
	defer server3.Close()

	t.Setenv("CONCUR_BASE_URL", server3.URL)

	cmd3 := RootCmd()
	cmd3.SetArgs([]string{
		"sync",
		"--resources", "expenses",
		"--user-id", "test-user-id",
		"--home", tempHome,
		"--no-prune",
		"--json",
	})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("third sync with --no-prune failed: %v", err)
	}

	// Verify database: exp-2 is STILL there because `--no-prune` was specified!
	db3, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db3.Close()

	var count3 int
	_ = db3.DB().QueryRow(`SELECT COUNT(*) FROM "expenses" WHERE expense_id = 'exp-2'`).Scan(&count3)
	if count3 != 1 {
		t.Errorf("expected exp-2 to be kept under --no-prune, but it was pruned (count = %d)", count3)
	}
}

// PATCH(amend-2026-11-04: expenses reconcile on every sync) -- a default
// (non --full) sync must drop an expense deleted in Concur, because each
// report is fetched as a complete listing.
func TestSync_ExpensesReconcileWithoutFull(t *testing.T) {
	tempHome := t.TempDir()
	expenseJSON := func(id string) string {
		return `{"expenseId":"` + id + `","transactionAmount":50.0,"expenseType":{"id":"01000"},"transactionDate":"2026-11-01"}`
	}
	runSync := func(t *testing.T, expenses ...string) {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/report-1/expenses") {
				_, _ = w.Write([]byte("[" + strings.Join(expenses, ",") + "]"))
				return
			}
			_, _ = w.Write([]byte(`{"links":[],"content":[{"reportId":"report-1"}]}`))
		}))
		defer server.Close()
		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
		cmd := RootCmd()
		cmd.SetArgs([]string{"sync", "--resources", "expenses", "--user-id", "test-user-id", "--home", tempHome, "--json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("sync failed: %v", err)
		}
	}
	countExpenses := func(t *testing.T) int {
		t.Helper()
		db, err := store.Open(filepath.Join(tempHome, "data", "data.db"))
		if err != nil {
			t.Fatalf("opening test db: %v", err)
		}
		defer db.Close()
		var n int
		_ = db.DB().QueryRow(`SELECT COUNT(*) FROM "expenses"`).Scan(&n)
		return n
	}

	runSync(t, expenseJSON("exp-1"), expenseJSON("exp-2"))
	if got := countExpenses(t); got != 2 {
		t.Fatalf("after first sync: %d expenses, want 2", got)
	}
	runSync(t, expenseJSON("exp-1"))
	if got := countExpenses(t); got != 1 {
		t.Errorf("after second sync without --full: %d expenses, want 1 (exp-2 deleted in Concur)", got)
	}
}
