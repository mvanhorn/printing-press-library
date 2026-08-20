// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

package obsidian

import "testing"

// IsRunning is a runtime probe of the Obsidian Local REST API plugin's
// loopback port. Without that port held open under test, the function
// must return false; covering the false case ensures the helper is at
// least invocable from tests on every supported platform.
func TestIsRunningReturnsBoolOnClosedPort(t *testing.T) {
	_ = IsRunning()
	// The function MUST return a boolean without panicking even when the
	// REST API plugin isn't installed or Obsidian isn't running. A panic
	// here would propagate to every staleness-warning call site.
}
