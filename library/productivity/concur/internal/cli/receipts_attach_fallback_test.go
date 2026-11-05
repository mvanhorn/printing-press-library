// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(amend-2026-11-03: Unit tests for receipts attach browser fallback and file validation)

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReceiptsAttach_FileValidation(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Missing file
	missingPath := filepath.Join(tempDir, "missing.pdf")

	// 2. Empty file
	emptyPath := filepath.Join(tempDir, "empty.pdf")
	if err := os.WriteFile(emptyPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Bad extension
	badExtPath := filepath.Join(tempDir, "bad.txt")
	if err := os.WriteFile(badExtPath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 4. Too large file (21 MB sparse file)
	largePath := filepath.Join(tempDir, "large.pdf")
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(21 * 1024 * 1024); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	tests := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "missing file error",
			filePath: missingPath,
			wantErr:  "does not exist",
		},
		{
			name:     "empty file error",
			filePath: emptyPath,
			wantErr:  "is empty",
		},
		{
			name:     "bad extension error",
			filePath: badExtPath,
			wantErr:  "unsupported file extension",
		},
		{
			name:     "too large error",
			filePath: largePath,
			wantErr:  "exceeds maximum allowed size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReceiptFile(tt.filePath)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestReceiptsAttach_BrowserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--session" ]; then
	arg1="$3"
	arg2="$4"
	arg3="$5"
fi

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/exp-1"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"upload-file":{"name":"upload","role":"file"}}}}'
	exit 0
fi

if [ "$arg1" = "upload" ]; then
	touch "$MOCK_STATE_FILE"
	exit 0
fi

exit 0
`
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)
	t.Setenv("MOCK_STATE_FILE", stateFile)

	// Create valid small file to attach
	receiptFile := filepath.Join(tmpDir, "receipt.pdf")
	if err := os.WriteFile(receiptFile, []byte("small content"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("HappyPath_SucceedsViaBrowser", func(t *testing.T) {
		_ = os.Remove(stateFile)

		// Set small poll parameters so tests are fast
		receiptsPollTimeout = 1 * time.Second
		receiptsPollInterval = 10 * time.Millisecond

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1"):
				// Main POST fails with 404 to trigger fallback
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"Not Found"}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				// fetchReportExpenses
				if _, err := os.Stat(stateFile); err == nil {
					// After browser upload: return exp-1 with a receiptImageId
					_, _ = w.Write([]byte(`[{"expenseId":"exp-1","receiptImageId":"img-456"}]`))
				} else {
					// Before: receiptImageId is null/empty
					_, _ = w.Write([]byte(`[{"expenseId":"exp-1","receiptImageId":null}]`))
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"receipts", "exp-1",
			"--file", receiptFile,
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected command execution error: %v", err)
		}

		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Error("expected browser fallback to be driven (upload invoked), but state file does not exist")
		}

		var res struct {
			ExpenseID      string `json:"expense_id"`
			ReportID       string `json:"report_id"`
			ReceiptImageID string `json:"receipt_image_id"`
			Via            string `json:"via"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal output: %v, output: %s", err, out.String())
		}

		if res.ExpenseID != "exp-1" || res.ReceiptImageID != "img-456" || res.Via != "browser" {
			t.Errorf("unexpected success response: %+v", res)
		}
	})

	t.Run("ExpenseNotFound_Fails", func(t *testing.T) {
		_ = os.Remove(stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1"):
				w.WriteHeader(http.StatusNotFound)
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				// Return list where exp-1 is missing
				_, _ = w.Write([]byte(`[{"expenseId":"exp-other","receiptImageId":null}]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"receipts", "exp-1",
			"--file", receiptFile,
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
		})

		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found in report") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("AlreadyHasReceipt_Refuses", func(t *testing.T) {
		_ = os.Remove(stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1"):
				w.WriteHeader(http.StatusNotFound)
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				// exp-1 already has receiptImageId
				_, _ = w.Write([]byte(`[{"expenseId":"exp-1","receiptImageId":"existing-img-id"}]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"receipts", "exp-1",
			"--file", receiptFile,
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
		})

		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "already has a receipt image") {
			t.Errorf("unexpected error: %v", err)
		}

		if _, err := os.Stat(stateFile); err == nil {
			t.Error("expected browser upload to NEVER be invoked when receipt already exists, but state file was touched")
		}
	})

	t.Run("404WithoutReportID_PrintsHint", func(t *testing.T) {
		_ = os.Remove(stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"Not Found"}`))
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"receipts", "exp-1",
			"--file", receiptFile,
			// No report-id supplied!
		})

		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "the receipts API returns 404 on this tenant; re-run with --report-id") {
			t.Errorf("unexpected error: %v", err)
		}

		if _, err := os.Stat(stateFile); err == nil {
			t.Error("expected browser upload to NEVER be invoked without --report-id, but state file was touched")
		}
	})

	t.Run("UploadOkButReceiptNeverAppears_PartialSuccess", func(t *testing.T) {
		_ = os.Remove(stateFile)

		// Set small poll parameters so timeout is fast
		receiptsPollTimeout = 100 * time.Millisecond
		receiptsPollInterval = 10 * time.Millisecond

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1"):
				w.WriteHeader(http.StatusNotFound)
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				// Return receiptImageId as null even after upload
				_, _ = w.Write([]byte(`[{"expenseId":"exp-1","receiptImageId":null}]`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"receipts", "exp-1",
			"--file", receiptFile,
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
		})

		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected partial success error, got nil")
		}

		var partialErr *receiptUploadPartialSuccessError
		if !errors.As(err, &partialErr) {
			t.Errorf("expected a receiptUploadPartialSuccessError, got %T: %v", err, err)
		}

		if !strings.Contains(err.Error(), "the browser fallback's receipt upload command completed but the receipt image has not yet appeared") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

// PATCH(amend-2026-11-04: test URL verification for receipts fallback)
func TestReceiptsAttach_BrowserFallback_WrongExpensePage(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--session" ]; then
	arg1="$3"
	arg2="$4"
fi

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	# Returns exp-999 (different expense) instead of exp-1!
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/exp-999"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"upload-file":{"name":"upload","role":"file"}}}}'
	exit 0
fi

if [ "$arg1" = "upload" ]; then
	touch "$MOCK_STATE_FILE"
	exit 0
fi

exit 0
`
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)
	t.Setenv("MOCK_STATE_FILE", stateFile)

	receiptFile := filepath.Join(tmpDir, "receipt.pdf")
	if err := os.WriteFile(receiptFile, []byte("small content"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/receiptimages/v1/exp-1"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessage":"Not Found"}`))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
			_, _ = w.Write([]byte(`[{"expenseId":"exp-1","receiptImageId":null}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"receipts", "exp-1",
		"--file", receiptFile,
		"--user-id", "test-user-id",
		"--report-id", "mock-report",
	})

	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when browser is on the wrong expense, got nil")
	}

	if !strings.Contains(err.Error(), "browser is not on the expected expense edit page") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Stat(stateFile); err == nil {
		t.Error("expected upload NOT to be invoked on wrong expense page, but state file exists")
	}
}
