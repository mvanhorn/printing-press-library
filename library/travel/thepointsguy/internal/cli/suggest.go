// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: search query suggestions from TPG's Algolia suggestions index.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSuggestCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "suggest <partial>",
		Short: "Get search query suggestions for a partial term",
		Long: strings.TrimSpace(`
Return query completions for a partial search term from The Points Guy's
Algolia query-suggestions index. Useful for discovering what to search for.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli suggest "amex"
  thepointsguy-pp-cli suggest "delta" --agent
`, "\n"),
		// Any string is a valid suggestion query; an unknown term yields an
		// empty (but successful) result, so there is no error-path to probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "amex", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch suggestions")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a partial search term is required"))
			}
			if limit <= 0 {
				limit = 10
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			sugs, err := c.Suggest(ctx, strings.Join(args, " "), limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Suggestions []string `json:"suggestions"`
				}{sugs})
			}
			for _, s := range sugs {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum suggestions to return")
	return cmd
}
