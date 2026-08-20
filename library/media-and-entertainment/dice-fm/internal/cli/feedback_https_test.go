// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Test for feedback https-only (Task 17).
package cli

import (
	"strings"
	"testing"
)

func TestPostFeedbackRejectsHTTP(t *testing.T) {
	err := postFeedback("http://example.com/feedback", FeedbackEntry{})
	if err == nil {
		t.Fatalf("postFeedback accepted cleartext http://; want rejection")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("error %q should mention the https requirement", err.Error())
	}
}
