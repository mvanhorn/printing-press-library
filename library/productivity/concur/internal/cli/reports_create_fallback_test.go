package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportsCreate_BrowserFallback(t *testing.T) {
	// Create a temp directory for our mock agent-browser script
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")

	// Set up the mock state file path
	stateFile := filepath.Join(tmpDir, "mock_state")

	// Write mock agent-browser script
	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"
arg3="$3"

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	if [ -f "$MOCK_STATE_FILE" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-fallback-report-12345:0"
	else
		echo "https://us2.concursolutions.com/nui/expense?confNum=new"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Report Name","role":"textbox"},"e2":{"name":"Business Purpose","role":"textbox"},"e3":{"name":"Create Report","role":"button"}}}}'
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e3" ]; then
	touch "$MOCK_STATE_FILE"
	exit 0
fi

exit 0
`
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}

	// Update PATH so our mock agent-browser is picked up
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)
	t.Setenv("MOCK_STATE_FILE", stateFile)

	// SCENARIO 1: policyId validation error triggers fallback
	t.Run("PolicyIdRequiredError_TriggersFallback", func(t *testing.T) {
		// Reset mock state file
		_ = os.Remove(stateFile)

		// Set up mock HTTP server
		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			if r.Method == "POST" && strings.Contains(r.URL.Path, "/reports") {
				// Pure HTTP first attempt: return 400 with policyId validation error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"validationErrors":[{"source":"policyId","message":"policyId is required","id":"field.required"}]}`))
				return
			}
			if r.Method == "GET" && strings.Contains(r.URL.Path, "/reports/mock-fallback-report-12345") {
				// Fallback HTTP get: return successful report details
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"mock-fallback-report-12345","name":"October Travel","businessPurpose":"Client site visit"}`))
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
			"reports", "create",
			"--user-id", "test-user-id",
			"--name", "October Travel",
			"--purpose", "Client site visit",
			"--yes",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if apiCalls != 2 {
			t.Errorf("expected 2 API calls (1 POST failing, 1 GET succeeding), got %d", apiCalls)
		}

		// Ensure mock state file was touched (meaning browser fallback actually click-submitted)
		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Error("expected browser fallback to be driven, but state file does not exist")
		}

		var envelope map[string]any
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("failed to unmarshal output JSON: %v", err)
		}

		if success, ok := envelope["success"].(bool); !ok || !success {
			t.Errorf("expected envelope success: true, got %+v", envelope)
		}
	})

	// SCENARIO 2: non-policyId error does NOT trigger fallback and surfaces original error
	t.Run("NonPolicyIdError_DoesNotTriggerFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"validationErrors":[{"source":"name","message":"Name is malformed","id":"field.invalid"}]}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"reports", "create",
			"--user-id", "test-user-id",
			"--name", "October Travel",
			"--yes",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if apiCalls != 1 {
			t.Errorf("expected exactly 1 API call, got %d", apiCalls)
		}

		if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
			t.Error("expected browser fallback NOT to be driven, but state file exists")
		}
	})

	// SCENARIO 3: successful HTTP create never touches the browser path
	t.Run("SuccessfulHTTP_NeverTriggersFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"direct-report-111","name":"Direct Report"}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"reports", "create",
			"--user-id", "test-user-id",
			"--name", "Direct Report",
			"--yes",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if apiCalls != 1 {
			t.Errorf("expected exactly 1 API call, got %d", apiCalls)
		}

		if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
			t.Error("expected browser fallback NOT to be driven, but state file exists")
		}
	})
}

// TestReportsCreate_BrowserFallback_DismissesInterstitial covers the bug
// found live 2026-09-04: a one-time-per-session promotional dialog (observed:
// "We've updated the hotel booking experience.") rendered on top of the
// Create Report modal and blocked the Create Report click. This exercises
// the happy path -- a plain "close" button click clears it on the first try,
// matching hotels_search.go's existing overlay-clearing convention -- and
// asserts the Escape fallback is NOT invoked when the simple click already
// worked.
func TestReportsCreate_BrowserFallback_DismissesInterstitial(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")
	closeClickedFile := filepath.Join(tmpDir, "close_clicked")
	escapeCalledFile := filepath.Join(tmpDir, "escape_called")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	if [ -f "$MOCK_STATE_FILE" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-interstitial-report-1:0"
	else
		echo "https://us2.concursolutions.com/nui/expense?confNum=new"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_CLOSE_CLICKED_FILE" ]; then
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Report Name","role":"textbox"},"e2":{"name":"Business Purpose","role":"textbox"},"e3":{"name":"Create Report","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e0":{"name":"close","role":"button"},"e1":{"name":"Report Name","role":"textbox"},"e2":{"name":"Business Purpose","role":"textbox"},"e3":{"name":"Create Report","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e0" ]; then
	touch "$MOCK_CLOSE_CLICKED_FILE"
	exit 0
fi

if [ "$arg1" = "press" ] && [ "$arg2" = "Escape" ]; then
	touch "$MOCK_ESCAPE_CALLED_FILE"
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e3" ]; then
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
	t.Setenv("MOCK_CLOSE_CLICKED_FILE", closeClickedFile)
	t.Setenv("MOCK_ESCAPE_CALLED_FILE", escapeCalledFile)

	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/reports") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"validationErrors":[{"source":"policyId","message":"policyId is required","id":"field.required"}]}`))
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/reports/mock-interstitial-report-1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"mock-interstitial-report-1","name":"August 2026 expense report"}`))
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
		"reports", "create",
		"--user-id", "test-user-id",
		"--name", "August 2026 expense report",
		"--yes",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(closeClickedFile); os.IsNotExist(err) {
		t.Error("expected the interstitial's close button to be clicked, but it wasn't")
	}
	if _, err := os.Stat(escapeCalledFile); !os.IsNotExist(err) {
		t.Error("Escape fallback should not fire when the close-button click already dismissed the interstitial")
	}
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("expected the flow to proceed to click Create Report after dismissing the interstitial")
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}
	if success, ok := envelope["success"].(bool); !ok || !success {
		t.Errorf("expected envelope success: true, got %+v", envelope)
	}
}

// TestReportsCreate_BrowserFallback_EscapeFallbackWhenCloseDoesNotDismiss
// covers the actual live failure mode found 2026-09-04: clicking the
// interstitial's "close" button reported success but did not visibly
// dismiss the dialog. This asserts the Escape fallback fires when a
// re-check still finds the dialog present, and that the flow still
// completes successfully afterward.
func TestReportsCreate_BrowserFallback_EscapeFallbackWhenCloseDoesNotDismiss(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")
	closeClickedFile := filepath.Join(tmpDir, "close_clicked")
	escapeCalledFile := filepath.Join(tmpDir, "escape_called")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	if [ -f "$MOCK_STATE_FILE" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-escape-report-1:0"
	else
		echo "https://us2.concursolutions.com/nui/expense?confNum=new"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_ESCAPE_CALLED_FILE" ]; then
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Report Name","role":"textbox"},"e2":{"name":"Business Purpose","role":"textbox"},"e3":{"name":"Create Report","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e0":{"name":"close","role":"button"},"e1":{"name":"Report Name","role":"textbox"},"e2":{"name":"Business Purpose","role":"textbox"},"e3":{"name":"Create Report","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e0" ]; then
	touch "$MOCK_CLOSE_CLICKED_FILE"
	exit 0
fi

if [ "$arg1" = "press" ] && [ "$arg2" = "Escape" ]; then
	touch "$MOCK_ESCAPE_CALLED_FILE"
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e3" ]; then
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
	t.Setenv("MOCK_CLOSE_CLICKED_FILE", closeClickedFile)
	t.Setenv("MOCK_ESCAPE_CALLED_FILE", escapeCalledFile)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/reports") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"validationErrors":[{"source":"policyId","message":"policyId is required","id":"field.required"}]}`))
			return
		}
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/reports/mock-escape-report-1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"mock-escape-report-1","name":"August 2026 expense report"}`))
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
		"reports", "create",
		"--user-id", "test-user-id",
		"--name", "August 2026 expense report",
		"--yes",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(closeClickedFile); os.IsNotExist(err) {
		t.Error("expected the close button to be tried first, but it wasn't clicked")
	}
	if _, err := os.Stat(escapeCalledFile); os.IsNotExist(err) {
		t.Error("expected Escape fallback to fire when the close-button click did not dismiss the dialog")
	}
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("expected the flow to still reach and click Create Report after the Escape fallback")
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}
	if success, ok := envelope["success"].(bool); !ok || !success {
		t.Errorf("expected envelope success: true, got %+v", envelope)
	}
}
