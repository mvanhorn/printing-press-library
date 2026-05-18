// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCouponsCmd_DryRun(t *testing.T) {
	var flags rootFlags
	flags.dryRun = true

	var buf bytes.Buffer
	cmd := newCouponsCmd(&flags)
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--dry-run should not error: %v", err)
	}
}

// TestMatchesStoreFilter unit-tests the merchant-substring logic used by the
// coupons command. End-to-end mocking is impractical because the slickdeals
// HTTP client is a surf-impersonated Chrome transport, not http.DefaultTransport;
// the command path is covered live by the v0.2 e2e probe script instead.
func TestMatchesStoreFilter(t *testing.T) {
	cases := []struct {
		name   string
		row    string
		needle string
		want   bool
	}{
		{"empty needle matches anything", `{"title":"x"}`, "", true},
		{"store field substring", `{"store":"amazon"}`, "amazon", true},
		{"merchant field substring", `{"merchant":"amazon.com"}`, "amazon", true},
		{"storeName field", `{"storeName":"Amazon"}`, "amazon", true},
		{"nested store object", `{"store":{"name":"Amazon"}}`, "amazon", true},
		{"no match across all fields", `{"store":"bestbuy"}`, "amazon", false},
		{"title fallback", `{"title":"20% Off at Amazon"}`, "amazon", true},
		{"invalid JSON returns false", `not json`, "amazon", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchesStoreFilter(json.RawMessage(c.row), c.needle)
			if got != c.want {
				t.Errorf("matchesStoreFilter(%q,%q)=%v want %v", c.row, c.needle, got, c.want)
			}
		})
	}
}
