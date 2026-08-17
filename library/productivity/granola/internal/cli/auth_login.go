// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

// PATCH(cli-owned-workos-session): `auth login` gives the CLI a session of its
// own via Granola's device authorization grant.
//
// This exists because there is no longer any borrowable credential on the
// machine. Granola moved the data encryption key into an entitlement-gated
// Keychain group, so supabase.json.enc cannot be read; the plaintext files it
// used to leave behind are months-stale; and the tokens the desktop app and the
// browser still hold are theirs -- refreshing one against the WorkOS endpoint
// consumes it and signs that client out. A CLI-owned chain is the only design
// that is durable without breaking something else.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign this CLI in to Granola (one-time; refreshes automatically after)",
		Long: "Signs this CLI in to Granola using the device authorization grant.\n\n" +
			"Opens a browser to a short approval page. After you approve once, the CLI\n" +
			"holds its own session and refreshes it silently on every later command --\n" +
			"you will not be asked again unless you run `auth logout`.\n\n" +
			"This does not touch the Granola desktop app's session or your browser's.",
		Example: "  granola-pp-cli auth login\n  granola-pp-cli auth login --no-browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			w := cmd.OutOrStdout()

			dc, err := granola.RequestDeviceCode(ctx)
			if err != nil {
				return err
			}

			target := dc.VerificationURIComplete
			if target == "" {
				target = dc.VerificationURI
			}

			if flags.asJSON || flags.agent {
				// Agent callers get the code as data and poll alongside us.
				b, _ := json.Marshal(map[string]any{
					"event":            "device_authorization_pending",
					"user_code":        dc.UserCode,
					"verification_uri": target,
					"expires_in":       dc.ExpiresIn,
				})
				fmt.Fprintln(w, string(b))
			} else {
				fmt.Fprintf(w, "\n  Approve this CLI at:  %s\n", target)
				fmt.Fprintf(w, "  Code:                 %s\n", dc.UserCode)
				fmt.Fprintf(w, "  (expires in %ds)\n\n", dc.ExpiresIn)
			}

			// Opening the browser is what makes this one click rather than a
			// copy-paste. Suppressed for agents and non-interactive runs, and
			// never fatal -- the URL is already printed above.
			if !noBrowser && !flags.noInput && !flags.agent && !flags.asJSON {
				if err := openBrowser(target); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  (could not open a browser: %v -- open the URL above)\n", err)
				}
			}

			session, err := granola.PollDeviceToken(ctx, dc)
			if err != nil {
				return err
			}

			// Persist before reporting success: a login we announce but did not
			// store is worse than one that visibly failed.
			if err := granola.SaveCLISession(session); err != nil {
				return fmt.Errorf("signed in, but could not save the session: %w", err)
			}
			granola.ResetTokenCache()

			if flags.asJSON || flags.agent {
				b, _ := json.Marshal(map[string]any{
					"event":   "device_authorization_complete",
					"account": session.AccountEmail,
					"stored":  granola.CLISessionPath(),
				})
				fmt.Fprintln(w, string(b))
				return nil
			}
			if session.AccountEmail != "" {
				fmt.Fprintf(w, "  Signed in as %s\n", session.AccountEmail)
			} else {
				fmt.Fprintln(w, "  Signed in.")
			}
			fmt.Fprintf(w, "  Session stored at %s (owner-only).\n", granola.CLISessionPath())
			fmt.Fprintln(w, "  Run `granola-pp-cli sync` to pull your meetings.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the approval URL without opening a browser")
	return cmd
}

// openBrowser opens a URL with the platform's handler.
func openBrowser(target string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, append(args, target)...).Start()
}
