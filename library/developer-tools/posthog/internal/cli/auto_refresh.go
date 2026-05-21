// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/posthog/internal/store"
)

// defaultStaleAfter is the threshold beyond which the local cache is
// considered stale and a background hint is shown.
const defaultStaleAfter = 24 * time.Hour

// autoRefreshIfStale opens the local store and, when key resources are older
// than staleAfter, prints a one-line hint to w encouraging the user to run
// sync. It never blocks, never returns an error that interrupts the command,
// and is a no-op when the store does not exist yet.
//
// This is wired into PersistentPreRunE so every read command benefits without
// each command needing its own freshness check.
func autoRefreshIfStale(ctx context.Context, w io.Writer, staleAfter time.Duration) {
	dbPath := defaultDBPath("posthog-pp-cli")
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		// Store does not exist yet — no hint needed.
		return
	}
	defer s.Close()

	keyResources := []string{"feature_flags", "experiments", "projects_dashboards"}
	if !cliutil.EnsureFresh(ctx, s, staleAfter, keyResources...) {
		fmt.Fprintf(w, "hint: local cache may be stale — run 'posthog-pp-cli sync' to refresh\n")
	}
}
