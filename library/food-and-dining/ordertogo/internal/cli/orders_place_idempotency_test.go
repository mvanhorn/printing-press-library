// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func checkoutTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if runtime.GOOS == "windows" {
		t.Skip("checkout is disabled on Windows until reservation metadata is crash safe")
	}
}

func testOrderBody(itemID int, subtotal float64) postOrderBody {
	return postOrderBody{Param: postOrderParam{
		RestName: "mixsushibarlin", RestID: 42,
		OrderDetails: orderDetails{Items: []cartItem{{ItemID: itemID, Price: subtotal}}, Subtotal: subtotal},
		Tax:          1.1,
		PaymentCard:  paymentCard{StripeCustomer: "cus_test", DefaultCardMap: map[string]any{"key": "card_test"}, Tip: 2, BillingAddress1: "First address"},
		Context:      map[string]any{"source": "test"},
	}}
}

func fingerprintBody(t *testing.T, body postOrderBody) string {
	t.Helper()
	fingerprint, err := cartFingerprint(body)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func testFingerprint(t *testing.T, itemID int, subtotal float64) string {
	t.Helper()
	return fingerprintBody(t, testOrderBody(itemID, subtotal))
}

func TestReservationBlocksRetryAfterUnknownOutcome(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)

	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	first.Release()

	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "unknown outcome") {
		t.Fatalf("uncertain retry got err %v, want unknown-outcome refusal", err)
	}
}

func TestReservationFingerprintCoversSubmittedOrder(t *testing.T) {
	base := fingerprintBody(t, testOrderBody(1, 5))
	for name, change := range map[string]func(*postOrderBody){
		"tax":             func(b *postOrderBody) { b.Param.Tax = 0.6 },
		"customer":        func(b *postOrderBody) { b.Param.PaymentCard.StripeCustomer = "cus_other" },
		"card":            func(b *postOrderBody) { b.Param.PaymentCard.DefaultCardMap["key"] = "card_other" },
		"billing address": func(b *postOrderBody) { b.Param.PaymentCard.BillingAddress1 = "Other address" },
		"order context":   func(b *postOrderBody) { b.Param.Context["source"] = "other" },
		"customer phone":  func(b *postOrderBody) { b.Param.CustomerPhone = "other" },
	} {
		body := testOrderBody(1, 5)
		change(&body)
		if fingerprintBody(t, body) == base {
			t.Fatalf("fingerprint ignores %s", name)
		}
	}
}

func TestReservationBlocksDifferentCartAfterUnknownOutcome(t *testing.T) {
	checkoutTestHome(t)
	a, err := reservePlacement(testFingerprint(t, 7, 12.5))
	if err != nil {
		t.Fatal(err)
	}
	a.Release()
	if _, err := reservePlacement(testFingerprint(t, 8, 18.0)); err == nil || !strings.Contains(err.Error(), "different details has an unknown outcome") {
		t.Fatalf("changed cart got err %v, want unknown-outcome refusal", err)
	}
}

func TestReservationFreshAfterConfirmedOrder(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.RequestID
	if warn := first.ConfirmSuccess(); warn != "" {
		t.Fatalf("confirm warned: %s", warn)
	}
	first.Release()

	second, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if second.RequestID == firstID {
		t.Fatalf("post-success placement got id %q, want a fresh id", second.RequestID)
	}
}

func TestReservationBlocksConcurrentSameCartPlacement(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("overlapping same-cart placement got err %v, want lock-held refusal", err)
	}
	// A changed cart must not bypass the in-flight checkout lock.
	if _, err := reservePlacement(testFingerprint(t, 9, 3.0)); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("changed cart bypassed checkout lock: %v", err)
	}
}

func TestReservationFailsClosedWhenRecordCannotPersist(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	writeFailure := func(string, pendingPlace) error { return errors.New("simulated disk failure") }
	if _, err := reservePlacementWithWriter(fp, writeFailure); err == nil || !strings.Contains(err.Error(), "refusing to place the order") {
		t.Fatalf("persistence failure got err %v, want fail-closed refusal", err)
	}
}

// Old reservations must not silently expire into a new charge attempt.
func TestReservationBlocksOldRecordsWithoutExpiry(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.RequestID
	first.Release()

	stale := pendingPlace{RequestID: firstID, CartFingerprint: fp, At: time.Now().Add(-24 * time.Hour)}
	if err := writeFileDurable(pendingPlaceRecordPath(), stale); err != nil {
		t.Fatal(err)
	}
	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "unknown outcome") {
		t.Fatalf("day-old uncertain checkout got err %v, want refusal", err)
	}
}

func TestReservationFailsClosedOnCorruptRecord(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	r, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	r.Release()
	if err := os.WriteFile(pendingPlaceRecordPath(), []byte(`{"request_id":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("corrupt record got err %v, want fail-closed corruption error", err)
	}
}
