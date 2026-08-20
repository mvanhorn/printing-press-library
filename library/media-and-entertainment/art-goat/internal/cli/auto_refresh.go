// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
)

// freshnessMaxAge is the threshold past which the local corpus is
// considered stale and the user gets a passive nudge to run
// `sources sync`. A week balances "the user just opened the CLI for
// the first time after a vacation" against "every invocation nags
// them about a 6-hour-old corpus."
const freshnessMaxAge = 7 * 24 * time.Hour

// noAutoRefreshEnvVar lets a user (or a parent process — cron jobs,
// scripts) silence the staleness nudge without disabling sync entirely.
const noAutoRefreshEnvVar = "ART_GOAT_NO_AUTOREFRESH"

// autoRefreshIfStale is invoked from root PersistentPreRunE on every
// CLI invocation. It calls cliutil.EnsureFresh against the local
// works DB and, if the corpus has aged past freshnessMaxAge, prints
// a one-line stderr nudge pointing the user at `sources sync`.
//
// It deliberately does NOT shell out to perform the sync. A long
// network sync inside PreRunE would steal latency from short commands
// (and break determinism under the verifier). The passive nudge is
// the right shape; an explicit `sources sync` keeps refresh under the
// user's control.
//
// The returned bool reports whether a refresh was performed. Today
// that is always false (we only nudge); the signature leaves room
// for a future opt-in autorefresh without changing call sites.
//
// Short-circuits early when:
//   - cliutil.IsVerifyEnv() is true (verifier must never trigger
//     background work or read real SQLite files).
//   - PRINTING_PRESS_DOGFOOD=1 (dogfood matrix owns its per-command
//     timing budget; nudges add noise).
//   - ART_GOAT_NO_AUTOREFRESH=1 (explicit user opt-out).
func autoRefreshIfStale(ctx context.Context, dbPath string) (refreshed bool, err error) {
	_ = ctx // reserved for a future opt-in background refresh
	if cliutil.IsVerifyEnv() {
		return false, nil
	}
	if os.Getenv("PRINTING_PRESS_DOGFOOD") == "1" {
		return false, nil
	}
	if os.Getenv(noAutoRefreshEnvVar) == "1" {
		return false, nil
	}

	stale, _, err := cliutil.EnsureFresh(dbPath, freshnessMaxAge)
	if err != nil {
		// EnsureFresh errors are advisory; surfacing them in PreRunE
		// would block every command on a DB hiccup. Stay silent.
		return false, nil
	}
	if !stale {
		return false, nil
	}
	fmt.Fprintln(os.Stderr, "art-goat: corpus is stale; run `art-goat-pp-cli sources sync` to refresh")
	return false, nil
}
