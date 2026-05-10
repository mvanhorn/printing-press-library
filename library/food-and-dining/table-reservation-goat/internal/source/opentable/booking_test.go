// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"errors"
	"testing"
)

func TestExtractSlotLockID_FromURL(t *testing.T) {
	cases := []struct {
		url  string
		want int64
	}{
		{"https://www.opentable.com/booking/details?slotLockId=1074646572&rid=1255093", 1074646572},
		{"/booking/details?rid=1&slotLockId=42&other=x", 42},
		{"https://example.com/no-lock-here", 0},
	}
	for _, tc := range cases {
		got := extractSlotLockID(tc.url, nil)
		if got != tc.want {
			t.Errorf("extractSlotLockID(%q) = %d; want %d", tc.url, got, tc.want)
		}
	}
}

func TestExtractSlotLockID_FromBody(t *testing.T) {
	body := []byte(`<html>...,"slotLockId=999888777","other":...</html>`)
	got := extractSlotLockID("", body)
	if got != 999888777 {
		t.Errorf("extractSlotLockID body = %d; want 999888777", got)
	}
}

func TestExtractSlotLockID_PrefersURL(t *testing.T) {
	// URL is the canonical source — body match should not override.
	url := "/x?slotLockId=111"
	body := []byte(`slotLockId=222`)
	got := extractSlotLockID(url, body)
	if got != 111 {
		t.Errorf("extractSlotLockID with URL+body = %d; want 111 (URL preferred)", got)
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	// Verify each sentinel is identifiable via errors.Is so the CLI
	// can map each to a distinct JSON error category.
	wrapped := []struct {
		name string
		err  error
		base error
	}{
		{"slot-taken", errors.Join(ErrSlotTaken, errors.New("HTTP 409")), ErrSlotTaken},
		{"payment-required", errors.Join(ErrPaymentRequired, errors.New("HTTP 402")), ErrPaymentRequired},
		{"auth-expired", errors.Join(ErrAuthExpired, errors.New("HTTP 401")), ErrAuthExpired},
		{"past-window", errors.Join(ErrPastCancellationWindow, errors.New("statusCode 410")), ErrPastCancellationWindow},
		{"canary", errors.Join(ErrCanaryUnrecognizedBody, errors.New("decode failure")), ErrCanaryUnrecognizedBody},
	}
	for _, tc := range wrapped {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.base) {
				t.Errorf("errors.Is(%q, %v) = false; sentinel must be retrievable via errors.Is", tc.err, tc.base)
			}
			// Cross-check: each sentinel should NOT match a different sentinel.
			others := []error{ErrSlotTaken, ErrPaymentRequired, ErrAuthExpired, ErrPastCancellationWindow, ErrCanaryUnrecognizedBody}
			for _, o := range others {
				if o == tc.base {
					continue
				}
				if errors.Is(tc.err, o) {
					t.Errorf("errors.Is(%q, %v) = true; sentinels should be distinct", tc.err, o)
				}
			}
		})
	}
}

func TestPersistedHashesAreSet(t *testing.T) {
	if len(CancelReservationHash) != 64 {
		t.Errorf("CancelReservationHash = %q (len %d); expected 64-char SHA256", CancelReservationHash, len(CancelReservationHash))
	}
	if len(BookingConfirmationHash) != 64 {
		t.Errorf("BookingConfirmationHash = %q (len %d); expected 64-char SHA256", BookingConfirmationHash, len(BookingConfirmationHash))
	}
}
