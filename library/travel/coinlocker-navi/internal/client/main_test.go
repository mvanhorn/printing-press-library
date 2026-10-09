// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil/testenv"
)

// Client tests build real clients whose cache dir comes from the user's
// home; a mutation test then removes <cache>/http/resources. Run every test
// in this package in a temporary home so the user's real cache survives.
func TestMain(m *testing.M) {
	os.Exit(testenv.RunSandboxed(m))
}
