// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage your OfferUp login session (for account commands)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuthLoginCmd(flags), newAuthStatusCmd(flags), newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var chrome bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Capture your OfferUp session via a controlled Chrome window",
		Long: "Opens a controlled Chrome window (separate from your daily profile) for a one-time\n" +
			"OfferUp login, then captures and encrypts the session for the authenticated\n" +
			"commands (account, my-listings, saved, messages). Requires the 'press-auth' companion.",
		Example:     "  offerup-pp-cli auth login --chrome",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Never open a browser under the verify/dogfood harness.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would open a controlled Chrome window to capture your OfferUp session")
				return nil
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Opening a controlled Chrome window — log into OfferUp there; it captures your session and closes.")
			if err := offerup.RunLogin(cmd.Context()); err != nil {
				return classifyOfferupError(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged in — your OfferUp session is captured.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&chrome, "chrome", false, "Capture via a controlled Chrome window (the only supported login method)")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Report whether an OfferUp session is captured",
		Example:     "  offerup-pp-cli auth status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"loggedIn":           offerup.LoggedIn(cmd.Context()),
				"pressAuthInstalled": offerup.PressAuthBin() != "",
			}, flags)
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "logout",
		Short:       "Forget the captured OfferUp session",
		Example:     "  offerup-pp-cli auth logout",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				return nil
			}
			if err := offerup.Logout(cmd.Context()); err != nil {
				return classifyOfferupError(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Forgot the captured OfferUp session.")
			return nil
		},
	}
}
