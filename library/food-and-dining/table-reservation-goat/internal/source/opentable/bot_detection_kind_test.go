// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestBotDetectionError_KindDiscriminatesRecoveryMessage verifies that
// Kind drives distinct human-readable recovery hints. Issue #406
// failure 5: agents that swallow the error string need to be told
// what to try next — and "operation_blocked" vs "session_blocked"
// produces very different next actions.
func TestBotDetectionError_KindDiscriminatesRecoveryMessage(t *testing.T) {
	op := &BotDetectionError{
		Kind:   BotKindOperationBlocked,
		Status: 403,
		Reason: "Akamai blocks opname=RestaurantsAvailability",
	}
	msg := op.Error()
	for _, want := range []string{
		"operation blocked by Akamai WAF",
		"sibling ops still work",
		"numeric restaurant ID",
		"restaurants list",
		"chromedp escape hatch",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("operation_blocked error missing %q\nfull: %s", want, msg)
		}
	}
	if strings.Contains(msg, "session_blocked") {
		t.Errorf("operation_blocked error must not advertise session_blocked recovery\nfull: %s", msg)
	}

	sess := &BotDetectionError{
		Kind:   BotKindSessionBlocked,
		Status: 403,
		Until:  time.Now().Add(10 * time.Minute),
		Streak: 2,
		Reason: "bootstrap 403",
	}
	msg = sess.Error()
	for _, want := range []string{
		"anti-bot cooldown",
		"kind=session_blocked",
		"auth login --chrome",
		"refresh Akamai cookies",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("session_blocked error missing %q\nfull: %s", want, msg)
		}
	}
}

// TestBotDetectionError_EmptyKindDefaultsToSession covers backward
// compat: errors constructed without setting Kind (e.g. from older
// code paths or third-party error wrapping) should default to the
// session-blocked recovery message, since that's the more cautious
// recovery — telling the user to refresh cookies never hurts, while
// telling them to "use a numeric ID" is wrong advice for a real
// session block.
func TestBotDetectionError_EmptyKindDefaultsToSession(t *testing.T) {
	e := &BotDetectionError{
		Status: 403,
		Until:  time.Now().Add(5 * time.Minute),
		Reason: "some 403",
	}
	msg := e.Error()
	if !strings.Contains(msg, "auth login --chrome") {
		t.Errorf("default Kind should produce session-blocked recovery message; got: %s", msg)
	}
}

// TestIsBotDetection_RoundTripsKind verifies that callers using
// IsBotDetection() (the type-assertion helper) can read Kind off the
// returned pointer. This is the agent's path to typed recovery
// branching.
func TestIsBotDetection_RoundTripsKind(t *testing.T) {
	original := &BotDetectionError{Kind: BotKindOperationBlocked, Status: 403}
	wrapped := error(original)
	bde, ok := IsBotDetection(wrapped)
	if !ok {
		t.Fatal("IsBotDetection should report true for *BotDetectionError")
	}
	if bde.Kind != BotKindOperationBlocked {
		t.Errorf("Kind round-trip failed: got %q, want %q", bde.Kind, BotKindOperationBlocked)
	}
}

// TestIsBotDetection_NotBotError pins the negative case so callers
// can rely on the boolean signal.
func TestIsBotDetection_NotBotError(t *testing.T) {
	other := errors.New("some unrelated error")
	if _, ok := IsBotDetection(other); ok {
		t.Error("IsBotDetection should report false for non-BotDetectionError")
	}
	if _, ok := IsBotDetection(nil); ok {
		t.Error("IsBotDetection should report false for nil")
	}
}
