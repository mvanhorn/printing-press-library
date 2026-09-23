// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestNovelCheckoutPreviewHelpWires smoke-tests that the checkout preview command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCheckoutPreviewHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"checkout", "preview", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout preview --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "preview"} {
		if !strings.Contains(help, want) {
			t.Fatalf("checkout preview --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestUnicaIsNeitherSubscriptionNorUltraFast is the regression guard for the
// bug this patch fixes: `checkout preview` treated "not a subscription store"
// as "ultra fast", so `--store unica` reported is_ultra_fast=true even though
// GET /features/stores says is_ultra_fast_delivery=false for that storefront.
//
// PATCH: store-cluster-truth.
func TestUnicaIsNeitherSubscriptionNorUltraFast(t *testing.T) {
	if isSubscriptionStore("unica") {
		t.Error("unica has no recurring basket")
	}
	if isUltraFastStore("unica") {
		t.Error("unica is not ultra-fast — only now and now-bebidas are")
	}
	if isUltraFastStore("pontual") {
		t.Error("the `pontual` alias must resolve to unica, which is not ultra-fast")
	}
	for _, name := range []string{"now", "now-bebidas", "6", "8"} {
		if !isUltraFastStore(name) {
			t.Errorf("%s should be ultra-fast", name)
		}
		if isSubscriptionStore(name) {
			t.Errorf("%s has no recurring basket", name)
		}
	}
	for _, name := range []string{"programada", "mensal", "1", "fresh", "2", "pet", "5"} {
		if !isSubscriptionStore(name) {
			t.Errorf("%s should be a subscription store", name)
		}
		if isUltraFastStore(name) {
			t.Errorf("%s is not ultra-fast", name)
		}
	}
}

// TestResolveSubdomainCoversEveryStorefront pins the subdomain mapping after it
// was folded into the single store table.
//
// PATCH: store-cluster-truth.
func TestResolveSubdomainCoversEveryStorefront(t *testing.T) {
	cases := map[string]string{
		"programada": "programada", "mensal": "programada", "1": "programada",
		"fresh": "fresh", "2": "fresh",
		"unica": "unica", "pontual": "unica", "3": "unica",
		"pet": "pet", "5": "pet",
		"now": "now", "6": "now",
		"now-bebidas": "now-bebidas", "nowbebidas": "now-bebidas", "8": "now-bebidas",
		"": "programada", "bogus": "programada", "99": "programada",
	}
	for in, want := range cases {
		if got := resolveSubdomain(in); got != want {
			t.Errorf("resolveSubdomain(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCheckoutURLNeverHasEmptySubdomain pins the host label used by checkout
// preview. An unknown numeric id used to resolve with an empty subdomain and
// produce https://.shopper.com.br/shop/checkout.
func TestCheckoutURLNeverHasEmptySubdomain(t *testing.T) {
	for _, in := range []string{"", "99", "bogus", "unica", "programada", "1", "now"} {
		sub := resolveSubdomain(in)
		if sub == "" {
			t.Errorf("resolveSubdomain(%q) is empty", in)
		}
		url := "https://" + sub + ".shopper.com.br/shop/checkout"
		if strings.HasPrefix(url, "https://.") || strings.Contains(url, "://.") {
			t.Errorf("checkout URL for %q = %s", in, url)
		}
	}
}

// TestSubscriptionDeliveryFailureDoesNotRelabelAsOneOff is the P1 guard for
// checkout preview: a /delivery/summary failure on programada, fresh, or pet
// must not overwrite notes with the one-off/unica label, and must not attach
// a charge calendar.
func TestSubscriptionDeliveryFailureDoesNotRelabelAsOneOff(t *testing.T) {
	authNote := "cart, delivery, and payment data unavailable — authenticate first: shopper-pp-cli auth set-token <token>"
	next := &chargeCalendarEntry{DeliveryDate: "2026-10-01"}
	for _, store := range []string{"programada", "fresh", "pet", "mensal", "1", "5"} {
		view := checkoutPreviewView{Store: store, Note: authNote}
		applyCheckoutStoreMode(&view, store, errors.New("delivery summary failed"), next)
		if view.Note != authNote {
			t.Errorf("%s: note = %q, want the delivery-error note left untouched", store, view.Note)
		}
		if strings.Contains(view.Note, "One-off") || strings.Contains(view.Note, "unica") || strings.Contains(view.Note, "Ultra-fast") {
			t.Errorf("%s: delivery failure mislabeled the store: %q", store, view.Note)
		}
		if view.ChargeCalendar != nil {
			t.Errorf("%s: charge calendar set despite delivery error", store)
		}
		if view.Delivery != nil {
			t.Errorf("%s: delivery struct invented on a subscription error: %+v", store, view.Delivery)
		}
	}
}

// TestSubscriptionDeliverySuccessKeepsCalendar attaches the calendar only when
// delivery summary succeeded, and does not replace an existing note.
func TestSubscriptionDeliverySuccessKeepsCalendar(t *testing.T) {
	next := &chargeCalendarEntry{DeliveryDate: "2026-10-01"}
	view := checkoutPreviewView{Store: "programada", Note: "keep-me"}
	applyCheckoutStoreMode(&view, "programada", nil, next)
	if view.ChargeCalendar != next {
		t.Fatalf("calendar = %+v, want the supplied next delivery", view.ChargeCalendar)
	}
	if view.Note != "keep-me" {
		t.Fatalf("note = %q, want it left untouched", view.Note)
	}
	if view.Delivery != nil && view.Delivery.IsUltraFast {
		t.Fatal("programada marked ultra-fast")
	}
}

// TestNonSubscriptionCheckoutKeepsModeLabel keeps the ultra-fast and one-off
// notes for stores that are not on a recurring cycle, including when delivery
// summary failed.
func TestNonSubscriptionCheckoutKeepsModeLabel(t *testing.T) {
	unica := checkoutPreviewView{Store: "unica"}
	applyCheckoutStoreMode(&unica, "unica", errors.New("delivery summary failed"), &chargeCalendarEntry{})
	if unica.Delivery == nil || unica.Delivery.IsUltraFast {
		t.Fatalf("unica delivery = %+v, want one-off and not ultra-fast", unica.Delivery)
	}
	if !strings.Contains(unica.Note, "One-off store (unica)") {
		t.Fatalf("unica note = %q", unica.Note)
	}
	if unica.ChargeCalendar != nil {
		t.Fatal("unica must not grow a charge calendar")
	}

	now := checkoutPreviewView{Store: "now"}
	applyCheckoutStoreMode(&now, "now", nil, nil)
	if now.Delivery == nil || !now.Delivery.IsUltraFast {
		t.Fatalf("now delivery = %+v, want ultra-fast", now.Delivery)
	}
	if !strings.Contains(now.Note, "Ultra-fast") {
		t.Fatalf("now note = %q", now.Note)
	}
}
