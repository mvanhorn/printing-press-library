package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
)

// newInternalCmd is the parent for commands that use Lunch Money's undocumented
// web-UI backend at api.lunchmoney.app. Auth is cookie-based; see `internal auth`
// for how to seed the cookie jar.
func newInternalCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "internal",
		Short: "Commands that use Lunch Money's undocumented web-UI backend (cookie auth)",
		Long: `Commands here speak the api.lunchmoney.app backend that the web UI uses.
The endpoints are reverse-engineered and may change without notice. Auth is
session-cookie based — seed with 'internal auth set-cookie' once, then refresh
happens automatically, or set LUNCHMONEY_INTERNAL_COOKIE for a non-persistent
session-cookie override.

This is intentionally separated from the public-API commands (api.lunchmoney.dev/v2)
because the auth model and endpoint shapes differ.`,
	}
	cmd.AddCommand(newInternalAuthCmd())
	cmd.AddCommand(newInternalRequestCmd(flags))
	addInternalExtras(cmd, flags)
	return cmd
}

func newInternalRequestCmd(flags *rootFlags) *cobra.Command {
	var method, body string
	cmd := &cobra.Command{
		Use:   "request <path>",
		Short: "Raw request against the internal API (uses the cookie jar + auto-refresh)",
		Example: `  internal request /rules
  internal request /assets
  internal request --method POST --body '{}' /auth/token/refresh
  internal request --method PUT --body '{"status":"cleared"}' /transactions/12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: internal request [--method M] [--body J] <path>")
			}
			var bodyArg any
			if body != "" {
				bodyArg = json.RawMessage(body)
			}
			// PATCH: Global --dry-run must preview internal requests before
			// cookie/session lookup so agents can inspect mutating calls safely.
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, strings.ToUpper(method), args[0], bodyArg)
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			status, raw, err := c.DoRaw(method, args[0], bodyArg)
			if err != nil && status == 0 {
				return err
			}
			fmt.Fprintf(os.Stderr, "HTTP %d\n", status)
			cmd.OutOrStdout().Write(raw)
			if len(raw) > 0 && raw[len(raw)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if status >= 400 {
				// PATCH: Command code must return errors instead of terminating
				// the process so tests, MCP wrappers, and --deliver cleanup run.
				return apiErr(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	cmd.Flags().StringVar(&body, "body", "", "JSON request body")
	return cmd
}

// PATCH: Shared dry-run envelope for hand-written internal-API commands.
func writeInternalDryRun(cmd *cobra.Command, method, path string, body any) error {
	payload := map[string]any{
		"dry_run": true,
		"success": false,
		"status":  0,
		"method":  strings.ToUpper(method),
		"path":    path,
	}
	if body != nil {
		payload["request_body"] = body
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
}

func newInternalAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the cookie jar for the internal API",
	}
	cmd.AddCommand(newInternalAuthSetCookieCmd())
	cmd.AddCommand(newInternalAuthStatusCmd())
	cmd.AddCommand(newInternalAuthClearCmd())
	return cmd
}

func newInternalAuthSetCookieCmd() *cobra.Command {
	var stdin bool
	cmd := &cobra.Command{
		Use:   "set-cookie [cookie-string]",
		Short: "Seed the internal-API cookie jar from a browser DevTools 'Cookie:' header",
		Long: `Open my.lunchmoney.app in your browser while logged in, open DevTools →
Network tab → click any /api request → copy the full 'Cookie:' request header value
and paste it here (or pipe via --stdin). The cookies are persisted to
~/.config/lunch-money-pp-cli/internal-cookies.json with 0600 permissions.

For non-persistent one-off auth, set LUNCHMONEY_INTERNAL_COOKIE to the same
Cookie header value instead of writing the jar.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var s string
			if stdin {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				s = strings.TrimSpace(string(b))
			} else if len(args) > 0 {
				s = args[0]
			} else {
				return fmt.Errorf("provide cookie string as arg or use --stdin")
			}
			c, err := internalapi.New(internalapi.DefaultCookiePath())
			if err != nil {
				return err
			}
			c.SetCookieString(s)
			fmt.Fprintln(cmd.OutOrStdout(), "ok: cookie jar updated at", internalapi.DefaultCookiePath())
			return nil
		},
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read cookie string from stdin")
	return cmd
}

func newInternalAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check whether the internal API has a usable session",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := internalapi.New(internalapi.DefaultCookiePath())
			if err != nil {
				return err
			}
			if !c.HasSession() {
				fmt.Fprintln(cmd.OutOrStdout(), "no session — run 'internal auth set-cookie' or set LUNCHMONEY_INTERNAL_COOKIE")
				return nil
			}
			// Try a low-cost call to verify
			status, _, err := c.DoRaw("GET", "/system/status", nil)
			if err != nil {
				return err
			}
			if status >= 400 {
				fmt.Fprintf(cmd.OutOrStdout(), "session present but probe returned %d — likely expired; re-run 'internal auth set-cookie' or refresh LUNCHMONEY_INTERNAL_COOKIE\n", status)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok: session valid")
			return nil
		},
	}
}

func newInternalAuthClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove the persisted cookie jar",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := internalapi.DefaultCookiePath()
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if strings.TrimSpace(os.Getenv(internalapi.EnvInternalCookie)) != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "ok: cleared %s (note: %s is still set)\n", path, internalapi.EnvInternalCookie)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok: cleared", path)
			return nil
		},
	}
}
