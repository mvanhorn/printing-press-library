// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestFingerprintMeeting_SameKey(t *testing.T) {
	ts := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	a := fingerprintMeeting("Forecasting Meeting", ts, []string{"PII_EMAIL_EXAMPLE_A", "PII_EMAIL_EXAMPLE_B"})
	b := fingerprintMeeting("forecasting meeting", ts, []string{"PII_EMAIL_EXAMPLE_B", "PII_EMAIL_EXAMPLE_A"})
	if a != b {
		t.Errorf("expected same fingerprint, got %q vs %q", a, b)
	}
	c := fingerprintMeeting("Forecasting Meeting", ts.AddDate(0, 0, 1), []string{"PII_EMAIL_EXAMPLE_A"})
	if c == a {
		t.Errorf("expected different fingerprint when day differs")
	}
}
