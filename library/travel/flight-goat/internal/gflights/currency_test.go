// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"strings"
	"testing"
)

func TestNormalizeCurrency(t *testing.T) {
	unit, code, err := normalizeCurrency(" gbp ")
	if err != nil {
		t.Fatalf("normalizeCurrency(GBP): %v", err)
	}
	if code != "GBP" || unit.String() != "GBP" {
		t.Fatalf("normalizeCurrency(GBP) = (%s, %s), want GBP", unit, code)
	}

	_, code, err = normalizeCurrency("")
	if err != nil {
		t.Fatalf("normalizeCurrency(empty): %v", err)
	}
	if code != "USD" {
		t.Fatalf("normalizeCurrency(empty) code = %q, want USD", code)
	}
}

func TestNormalizeCurrencyRejectsInvalidCodes(t *testing.T) {
	_, _, err := normalizeCurrency("peanuts")
	if err == nil {
		t.Fatal("normalizeCurrency(peanuts) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "ISO 4217") {
		t.Fatalf("error %q did not mention ISO 4217", err)
	}
}

func TestGoogleFlightsCurrencyHeaderIncludesCode(t *testing.T) {
	header := googleFlightsCurrencyHeader("EUR")
	if !strings.Contains(header, `"EUR"`) {
		t.Fatalf("currency header %q did not include EUR", header)
	}
}
