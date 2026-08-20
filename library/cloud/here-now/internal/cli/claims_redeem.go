// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: `claims redeem <slug>`. Claims an anonymous
// site (attaches it to your account permanently) using the token stored in the
// local vault. Not generated.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelClaimsRedeemCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "redeem <slug>",
		Short: "Claim an anonymous site permanently using its stored claim token",
		Long: strings.Trim(`
Reads the claim token recorded for <slug> by 'publish dir --anon' and POSTs it
to the claim endpoint, attaching the site to your authenticated account so it
no longer expires. Requires auth.
`, "\n"),
		Example: strings.Trim(`
  here-now-pp-cli claims redeem my-site
  here-now-pp-cli claims redeem my-site --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := args[0]
			if dryRunOK(flags) {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run":    true,
						"slug":       slug,
						"would_post": fmt.Sprintf("/api/v1/publish/%s/claim", slug),
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would claim %q via /api/v1/publish/%s/claim\n", slug, slug)
				return nil
			}

			db, err := openHereNowStore(cmd.Context(), flagDB)
			if err != nil {
				return err
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := redeemClaim(cmd.Context(), c, db, slug)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Claimed %q — it is now attached to your account permanently.\n", slug)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: ~/.local/share/here-now-pp-cli/data.db)")
	return cmd
}
