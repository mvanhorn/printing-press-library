// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestProductsGetIdBindsPositionalID(t *testing.T) {
	gotPath := executeReadCommandPath(t, []string{
		"--data-source", "live",
		"--no-cache",
		"--json",
		"products", "get-id", "prod_123",
	})
	if gotPath != "/v1/products/prod_123" {
		t.Fatalf("request path = %q, want /v1/products/prod_123", gotPath)
	}
}

func TestNestedGetCommandBindsPathParamsInUseOrder(t *testing.T) {
	gotPath := executeReadCommandPath(t, []string{
		"--data-source", "live",
		"--no-cache",
		"--json",
		"customers", "sources", "get-customers-customer-id", "cus_123", "src_456",
	})
	if gotPath != "/v1/customers/cus_123/sources/src_456" {
		t.Fatalf("request path = %q, want /v1/customers/cus_123/sources/src_456", gotPath)
	}
}

// TestNestedGetCommandBindsPathParamsWhenUseOrderReversesURLOrder guards the
// case where the Use placeholder order is the reverse of the URL path order:
// Use is "<id> <transfer>" but the URL is /transfers/{transfer}/reversals/{id}.
// A naive left-to-right-by-URL binding would swap the two, so this fails unless
// binding genuinely follows the Use string.
func TestNestedGetCommandBindsPathParamsWhenUseOrderReversesURLOrder(t *testing.T) {
	gotPath := executeReadCommandPath(t, []string{
		"--data-source", "live",
		"--no-cache",
		"--json",
		"transfers", "reversals", "get-transfers-transfer-id", "rev_123", "tr_456",
	})
	if gotPath != "/v1/transfers/tr_456/reversals/rev_123" {
		t.Fatalf("request path = %q, want /v1/transfers/tr_456/reversals/rev_123", gotPath)
	}
}

func executeReadCommandPath(t *testing.T, args []string) string {
	t.Helper()

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	t.Setenv("STRIPE_BASE_URL", srv.URL)
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_path")
	t.Setenv("STRIPE_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v): %v", args, err)
	}
	return gotPath
}
