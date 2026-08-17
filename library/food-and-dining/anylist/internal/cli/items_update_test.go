// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func TestVerifyItemUpdateIncludesBarcode(t *testing.T) {
	t.Parallel()

	updated := &store.ItemRow{
		Quantity:   "2",
		Details:    "fresh",
		ProductUpc: "049000028904",
	}
	err := verifyItemUpdate(updated, map[string]string{
		"quantity":    "2",
		"details":     "fresh",
		"product_upc": "049000028904",
	})
	if err != nil {
		t.Fatalf("verifyItemUpdate returned error for matching fields: %v", err)
	}
}

func TestVerifyItemUpdateRejectsBarcodeMismatch(t *testing.T) {
	t.Parallel()

	err := verifyItemUpdate(&store.ItemRow{ProductUpc: "000000000000"}, map[string]string{
		"product_upc": "049000028904",
	})
	if err == nil {
		t.Fatal("verifyItemUpdate returned nil for mismatched barcode")
	}
	if !strings.Contains(err.Error(), "barcode") || !strings.Contains(err.Error(), "049000028904") {
		t.Fatalf("error = %q, want barcode verification detail", err)
	}
}
