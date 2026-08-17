// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Onboarding helpers: a SimpleFIN-aware "no credentials" error and an `auth
// setup` override that teaches the actual claim flow (the generated auth setup
// has no setup URL for this spec and points only at set-token).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/config"
)

const simplefinOnboardSteps = `To connect:
  1. Create a connection and copy your setup token from the SimpleFIN Bridge:
       https://bridge.simplefin.org
     (or grab a reusable demo token: https://beta-bridge.simplefin.org/info/developers)
  2. Claim it (one-time — exchanges the setup token for a stored Access URL):
       simplefin-pp-cli auth claim <setup-token>

Already have an Access URL (https://user:pass@host/simplefin)? Use it directly:
       simplefin-pp-cli auth set-token <access-url>
     or:  export SIMPLEFIN_ACCESS_URL=<access-url>

Check status anytime: simplefin-pp-cli auth status`

// errNoSimplefinCredentials is the onboarding guidance shown when a command
// needs the SimpleFIN Access URL but none is configured. Returned via authErr
// (exit 4) so scripts can distinguish "not set up" from other failures.
func errNoSimplefinCredentials() error {
	return fmt.Errorf("no SimpleFIN credentials configured.\n\n%s", simplefinOnboardSteps)
}

// newSimplefinAuthSetupCmd replaces the generated `auth setup`, which reports
// "No setup URL is configured" and points only at set-token. This version
// teaches the real claim flow and the bridge URL.
func newSimplefinAuthSetupCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:         "setup",
		Short:       "Print steps for connecting your SimpleFIN account",
		Example:     "  simplefin-pp-cli auth setup",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Already configured? Say so instead of re-printing onboarding.
			if cfg, err := config.Load(flags.configPath); err == nil && cfg.AuthHeader() != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Already configured. Run 'simplefin-pp-cli sync' to pull your data, or 'auth status' for details.")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "SimpleFIN connection setup")
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), simplefinOnboardSteps)
			if launch {
				fmt.Fprintln(cmd.ErrOrStderr(), "would open: https://bridge.simplefin.org")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Print the bridge URL to open (no browser launched in non-interactive mode)")
	return cmd
}
