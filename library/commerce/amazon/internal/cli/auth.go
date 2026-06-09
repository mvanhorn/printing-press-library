package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/auth"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Amazon session cookies for the active profile",
		Long: strings.TrimSpace(`
Three flows, pick the one that fits how locked down your Chrome cookie store is:

  auth login          - read .amazon.com cookies via kooky (no prompt if Chrome is closed)
  auth paste          - paste a Cookie: header copied from DevTools (always works)
  auth import-file    - read a cookies JSON file exported by Cookie-Editor

Every flow writes cookies.json under the active profile's directory.`),
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthPasteCmd(), newAuthImportFileCmd(), newAuthStatusCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Read .amazon.com cookies from Chrome via kooky",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would read Chrome cookies for %s\n", app.Profile.Name)
				return nil
			}
			sess, err := auth.LoadFromChrome()
			if err != nil {
				return coded(ExitAuth, "%v\n\nFalling back to `amazon-pp-cli auth paste` or `auth import-file` will always work.", err)
			}
			return persistSession(cmd, app, sess)
		},
	}
}

func newAuthPasteCmd() *cobra.Command {
	var headerInline string
	cmd := &cobra.Command{
		Use:     "paste",
		Short:   "Paste a Cookie: header string copied from DevTools",
		Example: "  amazon-pp-cli auth paste --header 'session-id=...; at-main=...'\n  amazon-pp-cli auth paste  # interactive: reads from stdin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			header := headerInline
			if header == "" {
				fmt.Fprintln(cmd.OutOrStderr(), "paste your Amazon Cookie header (ends with EOF / Ctrl+D):")
				b, err := io.ReadAll(bufio.NewReader(os.Stdin))
				if err != nil {
					return coded(ExitUsage, "reading stdin: %v", err)
				}
				header = string(b)
			}
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would write cookies.json with %d-byte header\n", len(header))
				return nil
			}
			sess, err := auth.LoadFromPaste(header, "")
			if err != nil {
				return coded(ExitUsage, "%v", err)
			}
			return persistSession(cmd, app, sess)
		},
	}
	cmd.Flags().StringVar(&headerInline, "header", "", "Cookie header value (alternative to stdin)")
	return cmd
}

func newAuthImportFileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import-file <path>",
		Short: "Import a Chrome cookies JSON export (Cookie-Editor format)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would import cookies from %s\n", args[0])
				return nil
			}
			sess, err := auth.LoadFromFile(args[0])
			if err != nil {
				return coded(ExitUsage, "%v", err)
			}
			return persistSession(cmd, app, sess)
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Report whether the active profile has a usable session",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			path := app.Cfg.CookiesPath(app.Profile.Name)
			info := map[string]any{
				"profile":      app.Profile.Name,
				"cookies_path": path,
				"loaded":       false,
				"has_marker":   false,
				"cookie_count": 0,
			}
			if sess, err := auth.Load(path); err == nil {
				info["loaded"] = true
				info["cookie_count"] = len(sess.Cookies)
				info["has_marker"] = sess.HasMarker()
				info["saved_at"] = sess.SavedAt
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "profile:       %s\n", app.Profile.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "cookies_path:  %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "loaded:        %v\n", info["loaded"])
			fmt.Fprintf(cmd.OutOrStdout(), "has_marker:    %v\n", info["has_marker"])
			fmt.Fprintf(cmd.OutOrStdout(), "cookie_count:  %v\n", info["cookie_count"])
			return nil
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the active profile's cookies.json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			path := app.Cfg.CookiesPath(app.Profile.Name)
			if app.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would delete %s\n", path)
				return nil
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return coded(ExitConflict, "remove cookies.json: %v", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", path)
			return nil
		},
	}
}

func persistSession(cmd *cobra.Command, app *AppContext, sess *auth.Session) error {
	path := app.Cfg.CookiesPath(app.Profile.Name)
	if err := sess.Save(path); err != nil {
		return coded(ExitConflict, "save cookies.json: %v", err)
	}
	if app.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
			"profile":      app.Profile.Name,
			"cookies_path": path,
			"cookie_count": len(sess.Cookies),
			"has_marker":   sess.HasMarker(),
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "saved %d cookies to %s\n", len(sess.Cookies), path)
	if !sess.HasMarker() {
		fmt.Fprintln(cmd.OutOrStderr(), "warning: session is missing the at-main + session-id pair; you may not be logged in. Try `amazon-pp-cli doctor` to confirm.")
	}
	return nil
}
