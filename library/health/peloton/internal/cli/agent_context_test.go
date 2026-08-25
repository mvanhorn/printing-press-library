// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestAgentContextAuthModeAvoidsOAuthWording guards MINOR #4b from a live
// post-fix verification sweep: Peloton has no OAuth flow at all (just
// POST /auth/login once, then persist the session), but this auth.mode
// value previously said "oauth2_refresh" -- a residual from before the
// managed-auth wording cleanup that cost hours of dead-end debugging
// chasing a nonexistent external OAuth provisioning service. It must never
// contain any OAuth-flavored substring again.
func TestAgentContextAuthModeAvoidsOAuthWording(t *testing.T) {
	ctx := buildAgentContext(newRootCmd(&rootFlags{}))
	if ctx.Auth.Mode != "session_login" {
		t.Fatalf("Auth.Mode = %q, want %q", ctx.Auth.Mode, "session_login")
	}
}
