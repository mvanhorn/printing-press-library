// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/bound"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Unsetenv("PP_MCP_MAX_BYTES"); err != nil {
		panic(err)
	}
	bound.MaxBytes = bound.Budget()
	os.Exit(m.Run())
}
