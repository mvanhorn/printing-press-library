// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// setNovelWorkspace simulates --workspace for tests. The flag binds to the
// package-level novelWorkspace var (see novel_wiring.go), so tests set it
// directly and restore the prior value on cleanup.
func setNovelWorkspace(t *testing.T, v string) {
	t.Helper()
	prev := novelWorkspace
	novelWorkspace = v
	t.Cleanup(func() { novelWorkspace = prev })
}
