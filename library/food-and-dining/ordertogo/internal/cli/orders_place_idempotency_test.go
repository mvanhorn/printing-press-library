// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testFingerprint(itemID int, subtotal float64) string {
	return cartFingerprint("mixsushibarlin", 42, []cartItem{{ItemID: itemID, Price: subtotal}}, subtotal, 1.1, 2, "cus_test", "card_test")
}

func TestReservationReusesIDForSameCartRetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fp := testFingerprint(7, 12.5)

	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reused {
		t.Fatal("first placement must not reuse an id")
	}
	firstID := first.RequestID
	first.Release()

	second, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if !second.Reused || second.RequestID != firstID {
		t.Fatalf("retry got id %q (reused=%v), want %q reused", second.RequestID, second.Reused, firstID)
	}
}

func TestReservationFingerprintCoversPaymentIdentityAndTax(t *testing.T) {
	base := cartFingerprint("slug", 1, []cartItem{{ItemID: 1, Price: 5}}, 5, 0.5, 1, "cus_a", "card_a")
	for name, other := range map[string]string{
		"tax":      cartFingerprint("slug", 1, []cartItem{{ItemID: 1, Price: 5}}, 5, 0.6, 1, "cus_a", "card_a"),
		"customer": cartFingerprint("slug", 1, []cartItem{{ItemID: 1, Price: 5}}, 5, 0.5, 1, "cus_b", "card_a"),
		"card":     cartFingerprint("slug", 1, []cartItem{{ItemID: 1, Price: 5}}, 5, 0.5, 1, "cus_a", "card_b"),
	} {
		if other == base {
			t.Fatalf("fingerprint ignores %s", name)
		}
	}
}

func TestReservationFreshForDifferentCart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := reservePlacement(testFingerprint(7, 12.5))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	b, err := reservePlacement(testFingerprint(8, 18.0))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Release()
	if b.Reused || b.RequestID == a.RequestID {
		t.Fatalf("different cart got id %q (reused=%v), want a fresh id", b.RequestID, b.Reused)
	}
}

func TestReservationFreshAfterConfirmedOrder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fp := testFingerprint(7, 12.5)
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
	if second.Reused || second.RequestID == firstID {
		t.Fatalf("post-success placement got id %q (reused=%v), want a fresh id", second.RequestID, second.Reused)
	}
}

func TestReservationBlocksConcurrentSameCartPlacement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fp := testFingerprint(7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("overlapping same-cart placement got err %v, want lock-held refusal", err)
	}
	// A different cart is not blocked by this cart's lock.
	other, err := reservePlacement(testFingerprint(9, 3.0))
	if err != nil {
		t.Fatalf("different cart blocked: %v", err)
	}
	other.Release()
}

func TestReservationFailsClosedWhenRecordCannotPersist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// First reservation creates the config dir; make it read-only so the
	// record write fails, then require reservation (and thus the POST) to
	// refuse.
	fp := testFingerprint(7, 12.5)
	r, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	if warn := r.ConfirmSuccess(); warn != "" {
		t.Fatal(warn)
	}
	r.Release()
	dir := filepath.Dir(pendingPlaceRecordPath(fp))
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "refusing to place the order") {
		t.Fatalf("persistence failure got err %v, want fail-closed refusal", err)
	}
}

// Old reservations are reused deliberately — there is no auto-expiry. An
// uncertain attempt (POST fired, response lost, record never cleared) must
// keep presenting the same id no matter how much time passes: expiring it
// into a fresh id is exactly the double-charge path. Reusing an old id is
// always safe — the server either dedups it or, once its own dedup window
// has passed, places the order exactly once.
func TestReservationReusesOldRecordsWithoutExpiry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fp := testFingerprint(7, 12.5)
	first, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.RequestID
	first.Release()

	stale := pendingPlace{RequestID: firstID, CartFingerprint: fp, At: time.Now().Add(-24 * time.Hour)}
	if err := writeFileDurable(pendingPlaceRecordPath(fp), stale); err != nil {
		t.Fatal(err)
	}
	second, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if !second.Reused || second.RequestID != firstID {
		t.Fatalf("day-old uncertain record got id %q (reused=%v), want %q reused — auto-expiry would recreate the double-charge", second.RequestID, second.Reused, firstID)
	}
}

func TestReservationFailsClosedOnCorruptRecord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fp := testFingerprint(7, 12.5)
	r, err := reservePlacement(fp)
	if err != nil {
		t.Fatal(err)
	}
	r.Release()
	if err := os.WriteFile(pendingPlaceRecordPath(fp), []byte(`{"request_id":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reservePlacement(fp); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("corrupt record got err %v, want fail-closed corruption error", err)
	}
}
