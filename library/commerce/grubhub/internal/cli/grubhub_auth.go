// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
	"github.com/spf13/cobra"
)

// newAuthLoginCmd mints and stores an anonymous Grubhub bearer token. No
// credentials are required: it scrapes a fresh client id from grubhub.com and
// exchanges it for an anonymous session, exactly like the website. The friendly
// top-level commands (near/compare/dish/deals/pick) mint on demand, so this is
// only needed to use the raw `restaurants`/`geo` endpoint commands directly.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Mint and store an anonymous Grubhub token (no credentials needed)",
		Long: "Mint an anonymous Grubhub bearer token and store it in the config file. No account or API key is required.\n\n" +
			"The friendly commands (near, compare, dish, deals, pick) mint a token automatically, so you only need this to use the raw 'restaurants' and 'geo' endpoint commands directly.",
		Example:     "  grubhub-pp-cli auth login",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mint an anonymous Grubhub token")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			token, err := grubhub.EnsureToken(ctx, flags.configPath)
			if err != nil {
				return authErr(err)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, map[string]any{"status": "ok", "token_length": len(token)})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Minted an anonymous Grubhub token. Raw 'restaurants' and 'geo' commands are now authenticated.")
			return nil
		},
	}
}
