// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/airbnb"
	"github.com/spf13/cobra"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage your Airbnb login session (Chrome cookie import)",
		Long: `Manage the Airbnb login session used for private data (inbox, messages,
trips, wishlists). No password is ever handled: the session is imported from a
browser you are already logged in to.

  auth login --chrome              Import cookies from your local Chrome profile
  auth login --cookies "<paste>"   Import from a pasted Cookie header
  auth status                      Show whether a session is active
  auth logout                      Delete the stored session`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var useChrome bool
	var cookies string
	var profile string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Import your Airbnb session from Chrome or a pasted cookie header",
		Example: "  airbnb-outreach-pp-cli auth login --chrome\n" +
			"  airbnb-outreach-pp-cli auth login --chrome --profile \"Profile 1\"\n" +
			"  airbnb-outreach-pp-cli auth login --cookies \"_aat=...; _airbed_session_id=...\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sess := airbnb.LoadSession()
			var n int
			switch {
			case cookies != "":
				n = sess.ImportFromCookieString(cookies)
				sess.Source = "manual"
				if n == 0 {
					return usageErr(fmt.Errorf("no cookies parsed from --cookies value"))
				}
			case useChrome:
				var err error
				n, err = sess.ImportFromChrome(profile)
				if err != nil {
					return authErr(err)
				}
			default:
				return usageErr(fmt.Errorf("specify --chrome to import from Chrome, or --cookies \"<header>\" to paste"))
			}
			if err := sess.Save(); err != nil {
				return err
			}
			result := map[string]any{
				"status":         "logged_in",
				"cookies_stored": n,
				"authenticated":  sess.Authenticated(),
				"source":         sess.Source,
			}
			if !sess.Authenticated() {
				result["warning"] = "no _aat/_airbed_session_id cookie found — private commands may fail; are you logged in?"
			}
			if flags.asJSON {
				return flags.printJSON(cmd, result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s Imported %d Airbnb cookies (%s).\n", green("✓"), n, sess.Source)
			if !sess.Authenticated() {
				fmt.Fprintln(cmd.OutOrStdout(), yellow("  Warning: no login cookie detected — make sure you're logged in to airbnb.com."))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Import cookies from the local Chrome profile")
	cmd.Flags().StringVar(&cookies, "cookies", "", "Import from a pasted \"k=v; k2=v2\" Cookie header")
	cmd.Flags().StringVar(&profile, "profile", "Default", "Chrome profile directory name (e.g. \"Default\", \"Profile 1\")")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an Airbnb session is active",
		RunE: func(cmd *cobra.Command, args []string) error {
			sess := airbnb.LoadSession()
			status := map[string]any{
				"authenticated": sess.Authenticated(),
				"source":        sess.Source,
				"cookies":       len(sess.Cookies),
			}
			if !sess.ImportedAt.IsZero() {
				status["imported_at"] = sess.ImportedAt.Format("2006-01-02 15:04:05")
			}
			if flags.asJSON {
				return flags.printJSON(cmd, status)
			}
			if sess.Authenticated() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s Authenticated (%d cookies, source: %s)\n", green("✓"), len(sess.Cookies), sess.Source)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s Not authenticated. Run 'airbnb-outreach-pp-cli auth login --chrome'.\n", yellow("○"))
			}
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the stored Airbnb session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := airbnb.ClearSession(); err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"status": "logged_out"})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Session cleared.")
			return nil
		},
	}
}
