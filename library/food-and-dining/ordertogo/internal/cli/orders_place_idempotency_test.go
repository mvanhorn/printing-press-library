// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
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

func TestPendingCheckoutPrecedesTokenRefresh(t *testing.T) {
	checkoutTestHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("ORDERTOGO_CONFIG", "")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := "stripe_customer_id = \"cus_test\"\nstripe_default_card = \"card_test\"\ncustomer_firstname = \"Test\"\ncustomer_lastname = \"User\"\ncustomer_phone = \"2025550147\"\n"
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	cartPath := filepath.Join(t.TempDir(), "cart.json")
	if err := os.WriteFile(cartPath, []byte(`{"items":[{"id":7,"price":12.5}],"subtotal":12.5}`), 0o600); err != nil {
		t.Fatal(err)
	}

	called := false
	previous := refreshCheckoutToken
	refreshCheckoutToken = func(_ *config.Config) (string, error) {
		called = true
		return "", errors.New("synthetic token refresh failure")
	}
	t.Cleanup(func() { refreshCheckoutToken = previous })

	run := func() error {
		cmd := newOrdersPlaceCmd(&rootFlags{configPath: configPath})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--cart-file", cartPath, "--restaurant", "test-restaurant", "--restid", "42", "--confirm", "--max", "100", "--force"})
		return cmd.Execute()
	}

	// Authentication fails before a new record is created.
	if err := run(); err == nil || !strings.Contains(err.Error(), "synthetic token refresh failure") {
		t.Fatalf("first checkout error = %v, want token failure", err)
	}
	if !called {
		t.Fatal("token refresh was not reached for a new checkout")
	}
	if _, err := os.Stat(pendingPlaceRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("token failure created a pending checkout record: %v", err)
	}

	// A previous unknown outcome must take priority over a later token failure.
	if err := os.MkdirAll(filepath.Dir(pendingPlaceRecordPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFileDurable(pendingPlaceRecordPath(), pendingPlace{RequestID: "test-request", CartFingerprint: "different-details", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	called = false
	if err := run(); err == nil || !strings.Contains(err.Error(), "unknown outcome") || !strings.Contains(err.Error(), "inspect recent orders") {
		t.Fatalf("pending checkout error = %v, want recent-orders instruction", err)
	}
	if called {
		t.Fatal("token refresh ran despite an unknown checkout outcome")
	}
}
