// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/partitaiva24/internal/config"

	"github.com/spf13/cobra"
)

// newAuthSetCmd lets users paste the cookie + nonce harvested from their
// browser DevTools. WordPress nonces rotate roughly every 24 hours, so
// `auth set --nonce <new>` exists as a separate refresh path for the
// common case where the cookie is still valid but the nonce expired.
func newAuthSetCmd(flags *rootFlags) *cobra.Command {
	var cookieFlag string
	var nonceFlag string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Save cookie and/or X-WP-Nonce captured from your browser",
		Long: `Save the WordPress session cookie and X-WP-Nonce header captured
from a logged-in browser session.

How to capture both:
  1. Sign in to https://partitaiva24.cloud/.
  2. Open DevTools → Network tab. Reload any page in the app.
  3. Click any /api/v1/* request. Copy:
       Request Headers → Cookie               → --cookie '...'
       Request Headers → X-WP-Nonce            → --nonce '...'
  4. Run this command with both values.

Refresh just the nonce when you start seeing rest_cookie_invalid_nonce:
  partitaiva24-pp-cli auth set --nonce <new-nonce>
`,
		Example: `  partitaiva24-pp-cli auth set --cookie 'p24_logged_in_<hash>=...' --nonce 'abc1234567'
  partitaiva24-pp-cli auth set --nonce 'abc1234567'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cookieFlag == "" && nonceFlag == "" {
				return fmt.Errorf("provide --cookie, --nonce, or both")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			w := cmd.OutOrStdout()
			if cookieFlag != "" {
				cookieFlag = strings.TrimSpace(cookieFlag)
				if err := cfg.SaveCookie(cookieFlag); err != nil {
					return configErr(fmt.Errorf("saving cookie: %w", err))
				}
				fmt.Fprintf(w, "%s saved cookie (%d bytes)\n", green("OK"), len(cookieFlag))
			}
			if nonceFlag != "" {
				nonceFlag = strings.TrimSpace(nonceFlag)
				if err := cfg.SaveNonce(nonceFlag); err != nil {
					return configErr(fmt.Errorf("saving nonce: %w", err))
				}
				fmt.Fprintf(w, "%s saved X-WP-Nonce\n", green("OK"))
			}
			fmt.Fprintf(w, "Config: %s\n", cfg.Path)
			return nil
		},
	}

	cmd.Flags().StringVar(&cookieFlag, "cookie", "", "Full Cookie header value (p24_logged_in_<hash>=...)")
	cmd.Flags().StringVar(&nonceFlag, "nonce", "", "X-WP-Nonce header value (rotates ~daily)")
	cmd.Annotations = map[string]string{"mcp:hidden": "true"}
	return cmd
}
