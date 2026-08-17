// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"testing"
	"time"
)

func TestClientRetryBackoff(t *testing.T) {
	t.Parallel()
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for attempt, expected := range want {
		if got := clientRetryBackoff(attempt); got != expected {
			t.Errorf("clientRetryBackoff(%d) = %s, want %s", attempt, got, expected)
		}
	}
}
