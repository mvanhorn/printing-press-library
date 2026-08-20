// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command group: `claims`. Lists the local claim vault —
// every anonymous publish's slug, claim token, URL, and 24h expiry recorded by
// `publish dir --anon`. Header is a plain copyright line so regen-merge keeps
// this hand-authored file.

package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelClaimsCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "claims",
		Short: "List the local claim vault of anonymous publishes (slug, expiry, claim status)",
		Long: strings.Trim(`
Every anonymous publish (publish dir --anon) records its slug, claim token, URL,
and 24h expiry into a local vault, so you can make an expiring site permanent
later without hunting for the token in terminal scrollback.
`, "\n"),
		Example: strings.Trim(`
  here-now-pp-cli claims
  here-now-pp-cli claims --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			views, err := listClaimViews(db, time.Now())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), views, flags)
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	cmd.AddCommand(newNovelClaimsExpiringCmd(flags))
	cmd.AddCommand(newNovelClaimsRedeemCmd(flags))
	return cmd
}
