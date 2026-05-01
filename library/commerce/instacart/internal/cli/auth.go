package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage your Instacart login (Chrome cookie extraction)",
		Long: `Instacart has no public API for shopper cart actions, so the CLI rides on
the session you already have in Chrome. 'auth login' reads the session
cookies (including HttpOnly ones the browser hides from JavaScript) straight
out of Chrome's cookie database using the 'kooky' Go library. Nothing is
typed, no password is stored.

If Chrome is locked (the browser is running), quit Chrome first or use
'auth paste' to paste a Cookie header from devtools.`,
	}

	cmd.AddCommand(
		newAuthLoginCmd(),
		newAuthStatusCmd(),
		newAuthLogoutCmd(),
		newAuthPasteCmd(),
		newAuthImportFileCmd(),
	)
	return cmd
}

func newAuthImportFileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-file <path>",
		Short: "Import session cookies from a JSON file (browser-use export format)",
		Long: `Fallback for newer Chrome on macOS where kooky cannot decrypt cookies.
Export your cookies via 'browser-use cookies export' or any tool that
produces the standard array-of-cookies JSON format, then pass the path
here. This is also useful for CI or scripted setups where you want to
provide a pre-extracted session file.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := auth.ImportFromFile(args[0])
			if err != nil {
				return coded(ExitAuth, "%v", err)
			}
			if err := sess.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d cookies from %s\n", len(sess.Cookies), args[0])
			return nil
		},
	}
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Extract Instacart session cookies from Chrome",
		Long: `Reads Chrome's cookie database via kooky and saves the Instacart session
cookies to ~/.config/instacart/session.json (mode 0600). You must already be
logged in at https://www.instacart.com in Chrome.

If the command fails with "database is locked", quit Chrome completely
(cmd+Q) and re-run.`,
		Example: "  instacart auth login\n  # or, if Chrome is locked:\n  instacart auth paste",
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := auth.ImportFromChrome()
			if err != nil {
				return coded(ExitAuth, "%v", err)
			}
			if err := sess.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d cookies from Chrome\n", len(sess.Cookies))
			for _, c := range sess.Cookies {
				masked := maskCookieValue(c.Value)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s = %s\n", c.Name, masked)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nrun `instacart doctor` to verify")
			fmt.Fprintln(cmd.OutOrStdout(), "tip: `instacart history sync` will pull your past orders into the local store")
			fmt.Fprintln(cmd.OutOrStdout(), "     so future `add` commands can resolve items you have bought before.")
			return nil
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show the current session (if any)",
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			sess, err := auth.LoadSession()
			if err != nil {
				if asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"logged_in": false, "error": err.Error()})
					return coded(ExitAuth, "not logged in")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "not logged in")
				return coded(ExitAuth, "not logged in")
			}
			if asJSON {
				out := map[string]any{
					"logged_in":  true,
					"source":     sess.Source,
					"created_at": sess.CreatedAt,
					"cookies":    len(sess.Cookies),
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged in (source: %s, %d cookies, created %s)\n",
				sess.Source, len(sess.Cookies), sess.CreatedAt.Format("2006-01-02 15:04:05"))
			for _, c := range sess.Cookies {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", c.Name)
			}
			return nil
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the saved session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.ClearSession(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "session cleared")
			return nil
		},
	}
}

func newAuthPasteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "paste",
		Short: "Import session cookies from a pasted Cookie header",
		Long: `Fallback for when kooky can't read Chrome (different browser, locked
profile, or corporate policy). Open devtools on instacart.com, go to
Network, pick any request, copy the Cookie request header value, and paste
it here (ending with a blank line or Ctrl-D).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Paste the Cookie header value (end with blank line or Ctrl-D):")
			reader := bufio.NewReader(os.Stdin)
			var b strings.Builder
			for {
				line, err := reader.ReadString('\n')
				if err == io.EOF {
					b.WriteString(line)
					break
				}
				if err != nil {
					return err
				}
				trimmed := strings.TrimRight(line, "\r\n")
				if trimmed == "" {
					break
				}
				b.WriteString(line)
			}
			sess, err := auth.ImportFromHeader(b.String())
			if err != nil {
				return coded(ExitAuth, "%v", err)
			}
			if err := sess.Save(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d cookies from pasted header\n", len(sess.Cookies))
			return nil
		},
	}
}

func maskCookieValue(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "..." + v[len(v)-4:]
}
