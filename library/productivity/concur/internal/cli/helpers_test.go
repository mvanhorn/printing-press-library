// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/client"
)

// TestClassifyAPIError_ExpensesCreate404Signature covers F2: a fully-valid
// expenses-create request body was confirmed live to return this exact
// Spring "no static resource" 404 shape instead of 201 once every field
// passes client-side validation. The generic "run the list command" hint
// is actively misleading for a blocked POST, so this signature must get
// the more specific, accurate hint instead. Uses a real *client.APIError
// (not a plain errors.New string) since the classifier now extracts
// Method/Path directly rather than string-matching the formatted message
// (see TestClassifyAPIError_404MatchIsScopedToPOSTExpensesCollection).
func TestClassifyAPIError_ExpensesCreate404Signature(t *testing.T) {
	err := &client.APIError{
		Method:     "POST",
		Path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses",
		StatusCode: 404,
		Body:       `{"errorMessage":"No static resource /expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses."}`,
	}

	got := classifyAPIError(err, &rootFlags{})
	if got == nil {
		t.Fatal("expected a non-nil classified error")
	}

	msg := got.Error()
	if !strings.Contains(msg, "Concur-backend defect") {
		t.Errorf("expected the specific 404 hint, got: %s", msg)
	}
	if strings.Contains(msg, "Run the 'list' command") {
		t.Errorf("expected the specific hint to replace the generic one, but generic text is still present: %s", msg)
	}

	var typed *cliError
	if !errors.As(got, &typed) {
		t.Fatalf("expected a *cliError, got %T", got)
	}
	if typed.code != 3 {
		t.Errorf("expected exit code 3 (not found), got %d", typed.code)
	}
}

// TestClassifyAPIError_Generic404Unaffected is a regression guard: an
// unrelated 404 (e.g. a bad report ID on a GET) must still get the
// original generic hint, not the expenses-create-specific one.
func TestClassifyAPIError_Generic404Unaffected(t *testing.T) {
	err := &client.APIError{
		Method:     "GET",
		Path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/does-not-exist",
		StatusCode: 404,
		Body:       `{"errorMessage":"report not found"}`,
	}

	got := classifyAPIError(err, &rootFlags{})
	if got == nil {
		t.Fatal("expected a non-nil classified error")
	}

	msg := got.Error()
	if !strings.Contains(msg, "Run the 'list' command") {
		t.Errorf("expected the generic 404 hint to still apply, got: %s", msg)
	}
	if strings.Contains(msg, "Concur-backend defect") {
		t.Errorf("expensescreate-specific hint should not fire on an unrelated 404: %s", msg)
	}
}

// TestClassifyAPIError_404MatchIsScopedToPOSTExpensesCollection covers the
// Greptile review finding "404 Match Is Too Broad": the original check
// matched on the path merely CONTAINING "/expenses", which also matches
// GET/PATCH routes on an individual expense (".../expenses/{id}", e.g.
// apply-rules' PATCH or a plain expenses get) and the unrelated attendees
// routes that happen to share the substring. If one of those returns
// Spring's identical "No static resource" 404 for a totally different
// reason, the broad match would incorrectly claim a valid POST may have
// partially succeeded. Each case here returns the SAME 404 body; only
// Method/Path differ, isolating the fix to routing, not body content.
func TestClassifyAPIError_404MatchIsScopedToPOSTExpensesCollection(t *testing.T) {
	const sameBody = `{"errorMessage":"No static resource /expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses/exp1."}`

	tests := []struct {
		name       string
		method     string
		path       string
		wantCreate bool // true if the create-specific "Concur-backend defect" hint should fire
	}{
		{
			name:       "PATCH on an individual expense (e.g. apply-rules) does not match",
			method:     "PATCH",
			path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses/exp1",
			wantCreate: false,
		},
		{
			name:       "GET on an individual expense does not match",
			method:     "GET",
			path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses/exp1",
			wantCreate: false,
		},
		{
			name:       "GET on the expense collection (not a POST) does not match",
			method:     "GET",
			path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses",
			wantCreate: false,
		},
		{
			name:       "POST on the exact expense collection path matches",
			method:     "POST",
			path:       "/expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses",
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &client.APIError{Method: tt.method, Path: tt.path, StatusCode: 404, Body: sameBody}
			got := classifyAPIError(err, &rootFlags{})
			msg := got.Error()
			isCreateHint := strings.Contains(msg, "Concur-backend defect")
			if isCreateHint != tt.wantCreate {
				t.Errorf("method=%s path=%s: create-specific hint fired=%v, want %v (message: %s)", tt.method, tt.path, isCreateHint, tt.wantCreate, msg)
			}
		})
	}
}
