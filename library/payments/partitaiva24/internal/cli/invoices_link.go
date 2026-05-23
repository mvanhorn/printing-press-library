// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored helper that prints the partitaiva24 web UI deep-link
// for an invoice id. Shipped alongside `invoices create-safe` (which embeds
// the same URL in its enriched response) so the URL is one command away on
// any historical invoice, not just freshly-created ones.
//
// pp:novel-static-reference - builds a deterministic URL from a stable,
// curated URL pattern (https://partitaiva24.cloud/app/fatturazione/vendita/<id>/).
// No API call is needed: the pattern is documented reference data, and the
// command is a pure local string-builder over the user-supplied id.

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newInvoicesLinkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <id>",
		Short: "Print the partitaiva24 web UI URL for an invoice",
		Long: `Resolve the canonical partitaiva24 web UI deep-link for an invoice id.
Useful for opening a draft or charged invoice in the browser without leaving
the terminal:

  open "$(partitaiva24-pp-cli invoices link <id>)"

The URL pattern is stable across all invoices regardless of status:
https://partitaiva24.cloud/app/fatturazione/vendita/<id>/`,
		Example: `  partitaiva24-pp-cli invoices link 06d5e64f-4be9-11f1-89c5-0667992834eb
  partitaiva24-pp-cli invoices link 06d5e64f-4be9-11f1-89c5-0667992834eb --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			id := args[0]
			url := webURLForInvoice(id)
			// Default to bare URL on stdout so `open $(invoices link ID)` works.
			// Only emit JSON when the caller explicitly asks for it.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":      id,
					"web_url": url,
				}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	}
	return cmd
}

// silence "imported and not used" if the build trims this file at some point
var _ = json.Marshal
