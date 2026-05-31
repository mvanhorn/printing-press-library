// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built `auth login` subcommand implementing the Suno Clerk browser-auth
// flow. Kept separate from the generated auth.go so a regenerate of auth.go
// does not clobber it; auth.go wires this in via newAuthLoginCmd.

package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
	"github.com/spf13/cobra"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		useChrome bool
		rawCookie string
		directJWT string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Suno via Clerk (reads the __client cookie from Chrome by default)",
		Long: `Authenticate against Suno's Clerk backend and store a minted JWT.

Sources (pick one):
  --chrome          Read the __client cookie from your Chrome cookie store (default).
  --cookie <value>  Use a raw __client cookie value you supply directly.
  --jwt <jwt>       Store a JWT directly, skipping Clerk entirely.`,
		Example: `  suno-pp-cli auth login --chrome
  suno-pp-cli auth login --cookie "<__client value>"
  suno-pp-cli auth login --jwt "<jwt>"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			w := cmd.OutOrStdout()

			// --jwt: store directly, clear stale Clerk session material.
			if directJWT != "" {
				if dryRunOK(flags) {
					return nil
				}
				if !cliutil.LooksLikeJWT(directJWT) {
					return usageErr(fmt.Errorf("the value passed to --jwt does not look like a JWT (expected 3 dot-separated base64url segments)"))
				}
				if err := cfg.SaveSunoJWTOnly(directJWT); err != nil {
					return configErr(fmt.Errorf("saving jwt: %w", err))
				}
				return reportLogin(cmd, flags, cfg, "jwt")
			}

			// Resolve the __client cookie + device id from the chosen source.
			clientCookie := rawCookie
			deviceID := cfg.DeviceID()
			source := "cookie"
			if clientCookie == "" {
				// Default path: read from Chrome.
				source = "chrome"
				if dryRunOK(flags) {
					return nil
				}
				sess, rerr := auth.ReadChromeSession(cmd.Context())
				if rerr != nil {
					return authErr(fmt.Errorf("reading Chrome cookies: %w", rerr))
				}
				clientCookie = sess.ClientCookie
				if sess.DeviceID != "" {
					deviceID = sess.DeviceID
				}
				if clientCookie == "" {
					return authErr(fmt.Errorf("no __client cookie found in Chrome.\n" +
						"      Log in to suno.com in Chrome, or pass --cookie <__client value> / --jwt <jwt>."))
				}
			} else if dryRunOK(flags) {
				return nil
			}

			httpClient := &http.Client{Timeout: 20 * time.Second}

			sessionID, err := auth.ResolveSessionID(cmd.Context(), httpClient, clientCookie)
			if err != nil {
				return authErr(err)
			}
			jwt, err := auth.MintJWT(cmd.Context(), httpClient, clientCookie, sessionID)
			if err != nil {
				return authErr(err)
			}

			if err := cfg.SaveSunoSession(jwt, clientCookie, sessionID, deviceID); err != nil {
				return configErr(fmt.Errorf("saving session: %w", err))
			}
			fmt.Fprintf(w, "Resolved Clerk session %s\n", maskTail(sessionID))
			return reportLogin(cmd, flags, cfg, source)
		},
	}

	cmd.Flags().BoolVar(&useChrome, "chrome", true, "Read the __client cookie from the Chrome cookie store")
	cmd.Flags().StringVar(&rawCookie, "cookie", "", "Use this raw __client cookie value instead of reading Chrome")
	cmd.Flags().StringVar(&directJWT, "jwt", "", "Store this JWT directly and skip the Clerk flow")
	return cmd
}

func reportLogin(cmd *cobra.Command, flags *rootFlags, cfg *config.Config, source string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"logged_in":   true,
			"source":      source,
			"config_path": cfg.Path,
		}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Credentials saved to %s\n", cfg.Path)
	return nil
}

// maskTail shows only the last 6 characters of an identifier for display.
func maskTail(s string) string {
	if len(s) <= 6 {
		return "****"
	}
	return "****" + s[len(s)-6:]
}
