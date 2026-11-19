// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package bound

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// The typed-budget child deliberately verifies package initialization from its own env.
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		if err := os.Unsetenv("PP_MCP_MAX_BYTES"); err != nil {
			panic(err)
		}
		MaxBytes = Budget()
	}
	os.Exit(m.Run())
}
