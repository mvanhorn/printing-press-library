// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: auth login --cookie / --clearance / --chrome.
//
// The --chrome flag is intentionally limited: modern Chrome (130+ on macOS)
// encrypts its cookie DB with an app-bound scheme that third-party tools
// cannot decrypt reliably. The supported credential path is --cookie (paste
// the rapidapi-context-id value from DevTools → Application → Cookies) plus
// --clearance (the cf_clearance value) when the network is Cloudflare-gated.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/config"
	"github.com/spf13/cobra"
)

// newAuthLoginCmd adds `auth login` with --cookie / --clearance / --chrome.
// Credential values are written only into the CLI config file (0600 perms)
// and never printed to stdout.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var chrome bool
	var clearance string
	var cookie string
	cmd := &cobra.Command{
		Use:     "login [flags]",
		Short:   "Store a RapidAPI session cookie (and optionally a Cloudflare clearance cookie)",
		Example: "  rapidapi-pp-cli auth login --cookie <rapidapi-context-id value>\n  rapidapi-pp-cli auth login --cookie <value> --clearance <cf_clearance value>\n  rapidapi-pp-cli auth login --chrome",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chrome {
				// Modern Chrome app-bound encryption blocks DB-level import;
				// guide the user to the supported paste path instead.
				if cookie == "" {
					return fmt.Errorf("--chrome cannot read this Chrome build's encrypted cookie DB (app-bound encryption). Use --cookie with the rapidapi-context-id value from DevTools → Application → Cookies instead")
				}
			}
			if cookie == "" && clearance == "" {
				return fmt.Errorf("no credential provided: pass --cookie <value>, --clearance <cf_clearance>, or --chrome")
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cookie != "" {
				cfg.RapidapiCookie = cookie
				cfg.RapidapiCsrfToken = ""
				if err := cfg.SaveCookie(cookie); err != nil {
					return fmt.Errorf("saving credentials: %w", err)
				}
			}
			if clearance != "" {
				if err := cfg.SaveClearance(clearance); err != nil {
					return fmt.Errorf("saving clearance: %w", err)
				}
			}
			w := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{"authenticated": true, "source": "cookie-import", "config": cfg.Path}, flags)
			}
			fmt.Fprintln(w, green("Credentials saved"))
			fmt.Fprintf(w, "  Source: cookie import\n")
			fmt.Fprintf(w, "  Config: %s\n", cfg.Path)
			if clearance != "" {
				fmt.Fprintln(w, "  Cloudflare cf_clearance configured (for gated networks).")
			}
			fmt.Fprintln(w, "  The CSRF token is auto-fetched at request time.")
			return nil
		},
	}
	cmd.Flags().StringVar(&cookie, "cookie", "", "RapidAPI session cookie value (rapidapi-context-id)")
	cmd.Flags().StringVar(&clearance, "clearance", "", "Cloudflare cf_clearance cookie value (for gated networks)")
	cmd.Flags().BoolVar(&chrome, "chrome", false, "Attempt Chrome cookie import (limited on modern Chrome builds)")
	return cmd
}
