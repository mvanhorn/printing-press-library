// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	if warn := first.MarkConfirmed(123); warn != "" {
		t.Fatalf("confirm warned: %s", warn)
	}
	first.Release()

	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "order 123") {
		t.Fatalf("unacknowledged confirmed order did not block a new charge: %v", err)
	}
	if _, err := reservePlacementAcknowledging(fp, 999); err == nil || !strings.Contains(err.Error(), "order 123") {
		t.Fatalf("wrong acknowledged order ID did not block checkout: %v", err)
	}
	second, err := reservePlacementAcknowledging(fp, 123)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if second.RequestID == firstID {
		t.Fatalf("post-success placement got id %q, want a fresh id", second.RequestID)
	}
}

func TestReservationRejectsMismatchedConfirmedReceipt(t *testing.T) {
	checkoutTestHome(t)
	fp := testFingerprint(t, 7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	if warn := first.MarkConfirmed(123); warn != "" {
		t.Fatalf("confirm warned: %s", warn)
	}
	first.Release()
	if err := writeFileDurable(pendingPlaceRecordPath(), pendingPlace{RequestID: "other-order", CartFingerprint: fp, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := reservePlacementAcknowledging(fp, 123); err == nil || !strings.Contains(err.Error(), "do not match") {
		t.Fatalf("mismatched pending checkout was cleared: %v", err)
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
	writeFailure := func(path string, record pendingPlace) error {
		if err := writeFileDurable(path, record); err != nil {
			return err
		}
		return errors.New("simulated disk failure after a partial write")
	}
	if _, err := reservePlacementWithWriter(fp, writeFailure); err == nil || !strings.Contains(err.Error(), "refusing to place the order") {
		t.Fatalf("persistence failure got err %v, want fail-closed refusal", err)
	}
	if _, err := os.Stat(pendingPlaceRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("failed pre-POST reservation left a stale record: %v", err)
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
	configPath, cartPath := checkoutCommandFixture(t)

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

func checkoutCommandFixture(t *testing.T) (string, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configText := "stripe_customer_id = \"cus_test\"\nstripe_default_card = \"card_test\"\ncustomer_firstname = \"Test\"\ncustomer_lastname = \"User\"\ncustomer_phone = \"2025550147\"\n"
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	cartPath := filepath.Join(t.TempDir(), "cart.json")
	if err := os.WriteFile(cartPath, []byte(`{"items":[{"id":7,"price":12.5}],"subtotal":12.5}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, cartPath
}

func TestOrdersPlaceDryRunRedactsWithoutReservation(t *testing.T) {
	checkoutTestHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("ORDERTOGO_BASE_URL", "http://127.0.0.1:1")
	configPath, cartPath := checkoutCommandFixture(t)
	called := false
	previous := refreshCheckoutToken
	refreshCheckoutToken = func(_ *config.Config) (string, error) {
		called = true
		return "", errors.New("token refresh must not run in dry-run")
	}
	t.Cleanup(func() { refreshCheckoutToken = previous })
	flags := &rootFlags{configPath: configPath}
	cmd := newRootCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "--dry-run", "orders", "place", "--cart-file", cartPath, "--restaurant", "test-restaurant", "--restid", "42", "--confirm", "--max", "100"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run checkout: %v", err)
	}
	if called {
		t.Fatal("dry-run refreshed a payment token")
	}
	if !strings.Contains(stdout.String(), `"dry_run": true`) || !strings.Contains(stdout.String(), `"status": "would_post"`) {
		t.Fatalf("dry-run did not report a preview: %s", stdout.String())
	}
	for _, value := range []string{"cus_test", "card_test", "2025550147"} {
		if strings.Contains(stdout.String(), value) || strings.Contains(stderr.String(), value) {
			t.Fatalf("dry-run leaked a configured payment value")
		}
	}
	if _, err := os.Stat(pendingPlaceRecordPath()); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a pending checkout record: %v", err)
	}
	if _, err := os.Stat(placeAttemptPath()); !os.IsNotExist(err) {
		t.Fatalf("dry-run stamped checkout cooldown: %v", err)
	}
}

type checkoutOutputFailure struct{}

func (checkoutOutputFailure) Write([]byte) (int, error) {
	return 0, errors.New("synthetic output failure")
}

func TestConfirmedReceiptSurvivesOutputFailure(t *testing.T) {
	checkoutTestHome(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	configPath, cartPath := checkoutCommandFixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/m/api/postmicmeshorder" {
			t.Errorf("unexpected mock request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"transaction":{"orderid":42,"amount":12.34},"order":{"orderToken":"test_42"}}`)
	}))
	defer server.Close()
	t.Setenv("ORDERTOGO_BASE_URL", server.URL)
	previous := refreshCheckoutToken
	refreshCheckoutToken = func(_ *config.Config) (string, error) { return "", nil }
	t.Cleanup(func() { refreshCheckoutToken = previous })
	run := func(output io.Writer) error {
		cmd := newOrdersPlaceCmd(&rootFlags{configPath: configPath})
		cmd.SetOut(output)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--cart-file", cartPath, "--restaurant", "test-restaurant", "--restid", "42", "--confirm", "--max", "100", "--force"})
		return cmd.Execute()
	}
	if err := run(checkoutOutputFailure{}); err == nil || !strings.Contains(err.Error(), "synthetic output failure") {
		t.Fatalf("mock checkout output failure = %v", err)
	}
	if calls != 1 {
		t.Fatalf("mock checkout POSTs = %d, want one", calls)
	}
	receipt, exists, err := loadPlacementRecord(confirmedPlaceRecordPath())
	if err != nil || !exists || receipt.ConfirmedOrderID != 42 {
		t.Fatalf("confirmed receipt after output failure = %#v, exists=%v, err=%v", receipt, exists, err)
	}
	if err := run(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "order 42") {
		t.Fatalf("retry after output failure = %v, want confirmed-order block", err)
	}
	if calls != 1 {
		t.Fatalf("retry sent a second mock checkout POST: calls=%d", calls)
	}
}
