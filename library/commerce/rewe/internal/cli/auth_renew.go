// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

func newNovelAuthRenewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Keep the REWE shop session alive without a password by replaying the 30-day single-sign-on cookie.",
		Long: "Renews the REWE web shop session from the stored single-sign-on cookies (KEYCLOAK_IDENTITY,\n" +
			"valid 30 days and extended on every renewal). No password is sent. Every shop command does\n" +
			"this automatically when the session is gone; run it from a daily job to keep the 30-day\n" +
			"window rolling. Exit code 4 means a human must log in to rewe.de again (captcha + 2FA).",
		Example:     "  rewe-pp-cli auth renew --json",
		Annotations: map[string]string{"pp:data-source": "live", "pp:live-happy-path": "true", "pp:happy-args": "--json", "pp:verified-by-proof": "auth-renew-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth renew")
			}
			sh, err := rewe.Open()
			if err != nil {
				return reweErr(err)
			}
			if err := sh.Renew(cmd.Context()); err != nil {
				return reweErr(err)
			}
			syncGeneratedCredential(flags, sh.Session)
			out := map[string]any{
				"renewed":     true,
				"logged_in":   true,
				"sso_expires": sh.Session.SSOExpiry().Format(time.RFC3339),
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Session renewed. Single sign-on valid until %s.\n", sh.Session.SSOExpiry().Local().Format("2006-01-02 15:04"))
			})
		},
	}
	return cmd
}
