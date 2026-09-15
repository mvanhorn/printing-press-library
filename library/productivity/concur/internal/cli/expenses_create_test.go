// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestExpensesCreate_FlagBodyUsesLiveConfirmedShape covers F1: the flag-driven
// body builder used to send expenseTypeCode/transactionCurrencyCode/
// paymentTypeId/vendorDescription, all rejected live with HTTP 400
// "Unrecognized field". The live API's NewReportExpense model instead wants
// expenseType/paymentType/vendor as objects ({"code":...}, {"id":...},
// {"name":...}) and transactionAmount/transactionDate as flat values
// (unchanged). This asserts the corrected shape is actually sent on the wire.
func TestExpensesCreate_FlagBodyUsesLiveConfirmedShape(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if err := json.Unmarshal(raw, &capturedBody); err != nil {
			t.Fatalf("unmarshaling request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"expenseId": "exp-1"}`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "create",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--type", "CELPH",
		"--date", "2026-09-15",
		"--amount", "50",
		"--payment-type", "CASH",
		"--vendor", "on-call cell phone",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody == nil {
		t.Fatal("server never received a request body")
	}

	if _, present := capturedBody["expenseTypeCode"]; present {
		t.Errorf("expenseTypeCode must not be sent (rejected live as unrecognized), got body: %+v", capturedBody)
	}
	if _, present := capturedBody["transactionCurrencyCode"]; present {
		t.Errorf("transactionCurrencyCode must not be sent (not a valid top-level property live), got body: %+v", capturedBody)
	}
	if _, present := capturedBody["paymentTypeId"]; present {
		t.Errorf("paymentTypeId must not be sent (rejected live as unrecognized), got body: %+v", capturedBody)
	}
	if _, present := capturedBody["vendorDescription"]; present {
		t.Errorf("vendorDescription must not be sent (rejected live as unrecognized), got body: %+v", capturedBody)
	}

	expenseType, ok := capturedBody["expenseType"].(map[string]any)
	if !ok {
		t.Fatalf("expected expenseType to be an object, got: %+v", capturedBody["expenseType"])
	}
	if expenseType["code"] != "CELPH" {
		t.Errorf("expected expenseType.code=CELPH, got %+v", expenseType)
	}

	paymentType, ok := capturedBody["paymentType"].(map[string]any)
	if !ok {
		t.Fatalf("expected paymentType to be an object, got: %+v", capturedBody["paymentType"])
	}
	if paymentType["id"] != "CASH" {
		t.Errorf("expected paymentType.id=CASH, got %+v", paymentType)
	}

	vendor, ok := capturedBody["vendor"].(map[string]any)
	if !ok {
		t.Fatalf("expected vendor to be an object, got: %+v", capturedBody["vendor"])
	}
	if vendor["name"] != "on-call cell phone" {
		t.Errorf("expected vendor.name=%q, got %+v", "on-call cell phone", vendor)
	}

	if capturedBody["transactionAmount"] != 50.0 {
		t.Errorf("expected transactionAmount=50 (flat number, unchanged), got %+v", capturedBody["transactionAmount"])
	}
	if capturedBody["transactionDate"] != "2026-09-15" {
		t.Errorf("expected transactionDate=2026-09-15 (flat string, unchanged), got %+v", capturedBody["transactionDate"])
	}
}

// TestExpensesCreate_NonUSDCurrencyRejected covers F1's currency gap: no
// working currency-override field was found live. Per Greptile review
// ("Currency Override Is Ignored"), a non-USD --currency must be REJECTED
// before any write is attempted, not warned-about-then-silently-submitted
// in USD/the report default -- a caller relying on the requested currency
// has no way to detect a silent mismatch from a JSON/agent success
// response, since the old warning was stderr-only.
func TestExpensesCreate_NonUSDCurrencyRejected(t *testing.T) {
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"expenseId": "exp-1"}`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "create",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--type", "CELPH",
		"--date", "2026-09-15",
		"--amount", "50",
		"--currency", "GBP",
		"--payment-type", "CASH",
		"--vendor", "on-call cell phone",
		"--json",
	})

	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error rejecting the unsupported --currency, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be applied") {
		t.Errorf("expected an error explaining the currency override can't be applied, got: %v", err)
	}
	if apiCalls != 0 {
		t.Errorf("expected the write to be rejected before any API call, got %d calls", apiCalls)
	}
}

// TestExpensesCreate_BusinessPurposeFlag covers the --business-purpose flag:
// businessPurpose is a valid flat top-level property on the live API
// (confirmed in the same 34-property error that revealed expenseType/
// paymentType/vendor's object shapes), distinct from --vendor (Vendor
// Description is a separate Concur form field, confirmed live 2026-09-15).
// Setting it at creation time avoids ever needing expenses apply-rules'
// PATCH-based fill.
func TestExpensesCreate_BusinessPurposeFlag(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if err := json.Unmarshal(raw, &capturedBody); err != nil {
			t.Fatalf("unmarshaling request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"expenseId": "exp-1"}`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "create",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--type", "01000",
		"--date", "2026-09-15",
		"--amount", "50",
		"--payment-type", "CASH",
		"--vendor", "F45 Training Culver City",
		"--business-purpose", "gym",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedBody["businessPurpose"] != "gym" {
		t.Errorf("expected businessPurpose=\"gym\", got %+v", capturedBody["businessPurpose"])
	}
	// vendor (Vendor Description) and businessPurpose (Business Purpose)
	// are distinct fields -- confirm --vendor didn't get overwritten or
	// conflated with --business-purpose.
	vendor, ok := capturedBody["vendor"].(map[string]any)
	if !ok || vendor["name"] != "F45 Training Culver City" {
		t.Errorf("expected vendor.name to remain distinct from businessPurpose, got %+v", capturedBody["vendor"])
	}
}
