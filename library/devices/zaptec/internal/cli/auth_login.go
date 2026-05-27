// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newAuthLoginCmd implements `auth login` — the OAuth2 password-grant flow that
// exchanges a Zaptec portal username/password for a bearer token and persists
// it to the config file. This is the recommended way to authenticate; the older
// `auth set-token` path remains for pre-obtained tokens.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var username, password string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with your Zaptec username and password (OAuth2)",
		Long: `Exchange your Zaptec portal credentials for a bearer token via the OAuth2
password grant and cache it locally. The token is reused until it expires.

Credentials are read, in order of precedence, from:
  1. --username / --password flags
  2. ZAPTEC_USERNAME / ZAPTEC_PASSWORD environment variables
  3. an interactive prompt (password is masked)

The password is never written to disk — only the resulting bearer token is.`,
		Example: strings.Trim(`
  # Interactive (prompts for anything not supplied)
  zaptec-pp-cli auth login

  # Non-interactive via env vars (good for scripts/CI)
  ZAPTEC_USERNAME=you@example.com ZAPTEC_PASSWORD=secret zaptec-pp-cli auth login
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			if flags.dryRun {
				fmt.Fprintln(w, "[dry-run] would exchange Zaptec username/password for a bearer token at /oauth/token")
				return nil
			}

			if username == "" {
				username = os.Getenv("ZAPTEC_USERNAME")
			}
			if password == "" {
				password = os.Getenv("ZAPTEC_PASSWORD")
			}

			interactive := !flags.noInput && term.IsTerminal(int(os.Stdin.Fd()))
			if username == "" {
				if !interactive {
					return usageErr(fmt.Errorf("no username: pass --username, set ZAPTEC_USERNAME, or run in a terminal"))
				}
				fmt.Fprint(w, "Zaptec username (email): ")
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				username = strings.TrimSpace(line)
			}
			if password == "" {
				if !interactive {
					return usageErr(fmt.Errorf("no password: pass --password, set ZAPTEC_PASSWORD, or run in a terminal"))
				}
				fmt.Fprint(w, "Zaptec password: ")
				pw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(w)
				if err != nil {
					return fmt.Errorf("reading password: %w", err)
				}
				password = strings.TrimSpace(string(pw))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			token, expiry, err := client.PasswordGrant(cfg.BaseURL, username, password, nil)
			if err != nil {
				return err
			}
			if err := cfg.SaveTokens("", "", token, "", expiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}

			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"success":      true,
					"username":     username,
					"token_expiry": expiry.Format(time.RFC3339),
				}, flags)
			}
			fmt.Fprintf(w, "Logged in as %s. Token cached until %s.\n", username, expiry.Format("2006-01-02 15:04"))
			return nil
		},
	}

	cmd.Flags().StringVar(&username, "username", "", "Zaptec portal username (or set ZAPTEC_USERNAME)")
	cmd.Flags().StringVar(&password, "password", "", "Zaptec portal password (or set ZAPTEC_PASSWORD)")
	return cmd
}

// maybeAutoLogin transparently fetches a bearer token when no usable token is
// cached but ZAPTEC_USERNAME/ZAPTEC_PASSWORD are available in the environment.
// It is best-effort: any failure is swallowed so the subsequent request fails
// with the normal 401 guidance instead of blocking here. It never runs during
// --dry-run or under the printing-press verifier.
func maybeAutoLogin(f *rootFlags, cfg *config.Config) {
	if f.dryRun || cliutil.IsVerifyEnv() {
		return
	}
	// A usable, non-expired token already exists — nothing to do.
	if cfg.AuthHeader() != "" && (cfg.TokenExpiry.IsZero() || time.Now().Before(cfg.TokenExpiry)) {
		return
	}
	u := os.Getenv("ZAPTEC_USERNAME")
	p := os.Getenv("ZAPTEC_PASSWORD")
	if u == "" || p == "" {
		return
	}
	token, expiry, err := client.PasswordGrant(cfg.BaseURL, u, p, nil)
	if err != nil {
		return
	}
	cfg.AccessToken = token
	cfg.TokenExpiry = expiry
	_ = cfg.SaveTokens("", "", token, "", expiry)
}
