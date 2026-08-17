// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed feature: the SimpleFIN claim flow. Exchanges a base64 setup token for
// an Access URL and stores it. Wired as 'auth claim' from root.go.
//
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/config"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

func newAuthClaimCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim <setup-token>",
		Short: "Exchange a SimpleFIN setup token for an Access URL and save it",
		Long: "SimpleFIN's onboarding: paste the base64 setup token you got from the SimpleFIN Bridge (or\n" +
			"the reusable demo token at beta-bridge.simplefin.org/info/developers). This POSTs to the\n" +
			"decoded claim URL, receives a long-lived Access URL with embedded Basic-auth credentials,\n" +
			"and stores it (chmod 600, never logged). A setup token is one-time-use.\n\n" +
			"If you already have an Access URL, use 'auth set-token <access-url>' instead.",
		Example:     "  simplefin-pp-cli auth claim aHR0cHM6Ly9iZXRhLWJyaWRnZS5z...",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would claim setup token and store the Access URL")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would claim setup token and store the Access URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a setup token is required"))
			}
			accessURL, err := simplefin.ClaimSetupToken(ctx, args[0], flags.timeout)
			if err != nil {
				return authErr(err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveCredential(accessURL); err != nil {
				return fmt.Errorf("saving access URL: %w", err)
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{"status": "claimed", "stored": true})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Claimed and stored your SimpleFIN Access URL.")
			fmt.Fprintln(cmd.OutOrStdout(), "Next: simplefin-pp-cli sync --since 90d")
			return nil
		},
	}
	return cmd
}
