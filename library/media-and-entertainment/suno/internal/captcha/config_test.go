// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package captcha

import (
	"testing"
	"time"
)

func TestDefaultTimeoutAllowsApprovalTime(t *testing.T) {
	if DefaultTimeout < 180*time.Second {
		t.Errorf("DefaultTimeout=%s, want >=180s so the user can approve the macOS Chrome-access prompt", DefaultTimeout)
	}
}
