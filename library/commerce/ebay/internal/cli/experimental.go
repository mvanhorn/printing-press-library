// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// experimentalWarning prints a one-line stderr notice before a known-broken
// command runs. Used for bid and snipe, which depend on eBay's /bfl/placebid
// endpoint -- that endpoint redirects browser-cookie sessions to sign-in,
// so the three-step bid flow cannot complete from this CLI today. The
// commands stay in the binary so a future browser-CDP rewrite can revive
// them; until then, anyone who opts in deserves to know what they're
// running.
func experimentalWarning(cmd *cobra.Command, name string) {
	fmt.Fprintf(cmd.ErrOrStderr(),
		"Warning: %s is experimental and currently fails end-to-end. "+
			"eBay's anti-bot gate on /bfl/placebid blocks the cookie-based bid flow. "+
			"See README#known-limitations.\n",
		name)
}
