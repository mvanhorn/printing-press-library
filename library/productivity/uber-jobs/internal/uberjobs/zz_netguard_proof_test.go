// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNetGuardRefusesRealHosts proves the test binary cannot reach Uber:
// a probe at the real default base URL fails in the dialer, not on the wire.
func TestNetGuardRefusesRealHosts(t *testing.T) {
	c := NewClient(DefaultBaseURL, 5*time.Second, "", "")
	c.Limiter = nil
	_, _, err := c.ProbeTotal(context.Background(), Query{})
	if err == nil || !strings.Contains(err.Error(), "test net guard") {
		t.Fatalf("probe at %s: err = %v, want the test net guard refusal", DefaultBaseURL, err)
	}
	if !IsTransport(err) {
		t.Fatalf("guard refusal should surface as a transport error, got %T", err)
	}
}
