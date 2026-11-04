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
