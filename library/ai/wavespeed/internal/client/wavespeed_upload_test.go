// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"testing"
	"time"
)

// The upload deadline scales with payload size so a multi-megabyte image on a
// slow link is not cut off by the JSON-sized default timeout.
func TestUploadTransferAllowanceScalesWithSize(t *testing.T) {
	if got := uploadTransferAllowance(false, 25*1024*1024); got != 0 {
		t.Fatalf("non-upload allowance = %s, want 0", got)
	}
	if got := uploadTransferAllowance(true, 10*1024); got != time.Second {
		t.Fatalf("10 KiB allowance = %s, want 1s", got)
	}
	if got := uploadTransferAllowance(true, 25*1024*1024); got < 200*time.Second {
		t.Fatalf("25 MB allowance = %s, want >= 200s", got)
	}
}
