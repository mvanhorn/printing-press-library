// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/dreoauth"

	"github.com/spf13/cobra"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	// Credential resolution (highest priority first):
	//   1. --password-stdin: read from stdin until EOF (Docker pattern; no leak)
	//   2. --password / --username: flag values (warns to stderr; legacy mysql/curl style)
	//   3. $DREO_USERNAME / $DREO_PASSWORD env vars (canonical / MCP path)
	//   4. cfg.DreoUsername / cfg.DreoPassword from ~/.config/dreo-pp-cli/config.toml
	//
	// --password is supported because users coming from mysql/curl/wget
	// expect it, but it prints a stderr warning every time it's used.
	// The non-leak script-friendly path is --password-stdin:
	//   echo $PWD | dreo-pp-cli auth login --username me@x.com --password-stdin
	//   op read 'op://Personal/Dreo/password' | dreo-pp-cli auth login --password-stdin
	var (
		flagUsername  string
		flagPassword  string
		passwordStdin bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange Dreo email/password for an access token (cached for subsequent calls)",
		Example: `  # Recommended: env vars (no flag leak, no stdin).
  export DREO_USERNAME=me@example.com
  export DREO_PASSWORD='your-password'
  dreo-pp-cli auth login

  # Scriptable: pipe password from a secret store (no leak; Docker-style).
  op read 'op://Personal/Dreo/password' | dreo-pp-cli auth login --username me@example.com --password-stdin

  # Flag-supplied (insecure; prints a stderr warning). Plaintext appears
  # in 'ps aux', /proc/<pid>/cmdline, audit logs, and shell history.
  dreo-pp-cli auth login --username me@example.com --password 'your-password'`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			username := flagUsername
			if username == "" {
				username = os.Getenv("DREO_USERNAME")
			}
			if username == "" {
				username = cfg.DreoUsername
			}

			password := ""
			switch {
			case passwordStdin:
				if flagPassword != "" {
					return usageErr(fmt.Errorf("--password and --password-stdin are mutually exclusive; pick one"))
				}
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return usageErr(fmt.Errorf("reading password from stdin: %w", err))
				}
				// Trim trailing newline only; tolerate intentional internal whitespace.
				password = strings.TrimRight(string(raw), "\r\n")
				if password == "" {
					return usageErr(fmt.Errorf("--password-stdin received empty input"))
				}
			case flagPassword != "":
				password = flagPassword
				fmt.Fprintln(cmd.ErrOrStderr(),
					"warning: --password leaks the plaintext value into `ps`, /proc/<pid>/cmdline, "+
						"audit logs, and shell history. Prefer $DREO_PASSWORD env var or --password-stdin.")
			default:
				password = os.Getenv("DREO_PASSWORD")
				if password == "" {
					password = cfg.DreoPassword
				}
			}

			if username == "" || password == "" {
				return usageErr(fmt.Errorf("auth login needs a username and password. Recommended: export DREO_USERNAME and DREO_PASSWORD env vars. Alternatives: --password-stdin (pipe-friendly) or --username/--password flags (insecure, warns)"))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would log in: skipped under PRINTING_PRESS_VERIFY")
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would log in as %s against %s\n", username, cfg.BaseURL)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resp, err := dreoauth.Login(ctx, cfg.BaseURL, username, password)
			if err != nil {
				return authErr(fmt.Errorf("auth login failed: %w", err))
			}
			if resp.AccessToken == "" {
				return authErr(fmt.Errorf("auth login: empty access token in response"))
			}

			// Persist credentials + token + region (mode 0600). Credentials
			// are written so 401-aware re-login can mint a fresh bearer
			// when the cached one expires — Dreo issues no refresh_token,
			// so the user/pass pair is the only refresh material we have.
			cfg.DreoUsername = username
			cfg.DreoPassword = password
			expiry := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
			if resp.ExpiresIn <= 0 {
				expiry = time.Now().Add(30 * 24 * time.Hour) // sensible default
			}
			if err := cfg.SaveTokens("", "", resp.AccessToken, "", expiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}
			if resp.Region != "" {
				_ = cfg.SaveRegion(resp.Region)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"authenticated": true,
					"email":         username,
					"region":        resp.Region,
					"expires_in":    resp.ExpiresIn,
					"config":        cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s, region %s\n", username, resp.Region)
			fmt.Fprintf(cmd.OutOrStdout(), "Credentials and bearer token saved to %s (mode 0600); Dreo issues no refresh token, so the user/pass pair is kept on disk to re-mint the bearer when it expires.\n", cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagUsername, "username", "", "Dreo account email (overrides $DREO_USERNAME)")
	cmd.Flags().StringVar(&flagPassword, "password", "", "Dreo password (INSECURE: leaks into ps/cmdline/history; prefer env or --password-stdin)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin until EOF (Docker-style; no leak)")
	return cmd
}
