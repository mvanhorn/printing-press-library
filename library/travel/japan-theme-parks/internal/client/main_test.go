// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil/testenv"
)

// New() resolves the real user cache directory, and a successful mutating
// request evicts <cache>/http/resources even when NoCache is set. Without
// this sandbox, the verify short-circuit tests deleted the operator's HTTP
// cache on every `go test ./...`.
func TestMain(m *testing.M) {
	os.Exit(testenv.RunSandboxed(m))
}
