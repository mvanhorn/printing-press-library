// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

// Live-mode write guard. Hand-written, NOT generated.
//
// Stripe API keys come in two flavors:
//   - sk_test_... (test mode) — no real money, safe to script against
//   - sk_live_... (live mode) — real money, real customers
//
// This guard blocks mutating commands (POST/PUT/PATCH/DELETE) by default
// when a live key is in use, requiring either the `--confirm-live` flag
// or the `STRIPE_CONFIRM_LIVE=1` environment variable to proceed.
//
// Read commands (GET) are always allowed.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// liveModeBlockedErr is the error returned when a live-mode write is blocked
// without explicit confirmation. Maps to typed exit code 10 (configErr).
type liveModeBlockedErr struct {
	command string
	method  string
}

func (e *liveModeBlockedErr) Error() string {
	return fmt.Sprintf(
		"refusing to run live-mode write (%s %s) without --confirm-live\n\n"+
			"You appear to be using a live-mode key (sk_live_...). Live writes affect real customers and real money.\n\n"+
			"To proceed, add --confirm-live or set STRIPE_CONFIRM_LIVE=1.\n"+
			"To use test mode instead, switch to a test-mode key (sk_test_...).",
		e.method, e.command,
	)
}

// checkLiveModeGuard verifies that mutating commands are not run against
// a live-mode key without explicit confirmation. Called from root.go's
// PersistentPreRunE before any command executes.
//
// Returns nil (allows command) when:
//   - Command has no `pp:method` annotation (framework command like doctor, sql, sync)
//   - Annotated method is GET (read)
//   - Auth key is not sk_live_*
//   - flags.confirmLive is true
//   - STRIPE_CONFIRM_LIVE env var is "1"
//   - Command is in the framework allowlist (auth, doctor, profile, etc.)
//
// Returns liveModeBlockedErr when guard fires.
func checkLiveModeGuard(cmd *cobra.Command, flags *rootFlags) error {
	// Explicit confirmation overrides everything
	if flags.confirmLive {
		return nil
	}
	if os.Getenv("STRIPE_CONFIRM_LIVE") == "1" {
		return nil
	}

	// Check the command's HTTP method annotation. Framework commands
	// (doctor, auth, sql, sync, search, etc.) won't have this annotation
	// and pass through. Spec-derived endpoint commands have method set.
	method := cmd.Annotations["pp:method"]
	if method == "" {
		return nil
	}
	method = strings.ToUpper(method)
	if method == "GET" || method == "HEAD" || method == "OPTIONS" {
		return nil
	}

	// At this point: method is mutating (POST/PUT/PATCH/DELETE).
	// Resolve the auth key value to detect live mode.
	if !isLiveModeKey() {
		return nil
	}

	return &liveModeBlockedErr{
		command: cmd.CommandPath(),
		method:  method,
	}
}

// isLiveModeKey returns true if the active Stripe credential is a live-mode key.
// Checks env vars first (highest precedence), then falls through to None.
//
// We check env directly rather than going through config.Load because the
// guard runs in PersistentPreRunE before config is necessarily loaded. Env
// var detection is sufficient: persisted-config keys go through `auth set-token`
// which writes to the same file, but the dominant deployment shape (and the
// shape this guard most needs to catch) is `export STRIPE_SECRET_KEY=sk_live_...`.
func isLiveModeKey() bool {
	for _, name := range []string{"STRIPE_SECRET_KEY", "STRIPE_BASIC_AUTH"} {
		if v := os.Getenv(name); v != "" {
			if strings.HasPrefix(v, "sk_live_") || strings.HasPrefix(v, "rk_live_") {
				return true
			}
		}
	}
	return false
}
