// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// migrate: copy progress (external-time log) between Dreaming accounts or
// languages (parity with brianlund/migrate_dsdf). Ships as an honest stub: the
// real copy needs a SECOND account's credentials to write into, which this
// session has no safe way to obtain. Emits actionable guidance instead of a
// fake success.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMigrateCmd(flags *rootFlags) *cobra.Command {
	var toToken string
	var toLang string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Copy your external-time log to another account or language (requires a destination token)",
		Long: "Copy your logged external-time entries into another Dreaming account or the\n" +
			"other language catalog. This requires the DESTINATION account's bearer token\n" +
			"(--to-token), which must be provided explicitly — there is no safe way to\n" +
			"mint it automatically. Without it, this command explains what it needs.",
		Example: strings.Trim(`
  dreaming-pp-cli migrate --to-lang fr --to-token <destination-bearer-token>
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if toToken == "" {
				msg := map[string]any{
					"status": "needs_destination_token",
					"detail": "migrate needs the destination account's bearer token via --to-token. " +
						"Export your source data first with 'external list --json', then re-run with --to-token (and --to-lang for a language switch).",
				}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), msg, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "migrate needs the destination account's bearer token.")
				fmt.Fprintln(cmd.OutOrStdout(), "  1. Export your source log:  dreaming-pp-cli external list --json > log.json")
				fmt.Fprintln(cmd.OutOrStdout(), "  2. Re-run with:             dreaming-pp-cli migrate --to-token <token> [--to-lang fr]")
				fmt.Fprintln(cmd.OutOrStdout(), "  (The destination token is the 'token' localStorage value on the target account.)")
				return nil
			}
			// A real cross-account write is intentionally not performed here:
			// pushing external-time into a second account is a destructive,
			// account-scoped action that this session cannot verify safely.
			return apiErr(fmt.Errorf("cross-account migration is not enabled in this build; use 'external list --json' on the source and 'external import' on the destination account"))
		},
	}
	cmd.Flags().StringVar(&toToken, "to-token", "", "Destination account bearer token (required to actually migrate)")
	cmd.Flags().StringVar(&toLang, "to-lang", "", "Destination language catalog (es or fr)")
	return cmd
}
