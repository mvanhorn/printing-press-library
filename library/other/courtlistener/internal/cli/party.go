// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import (
	"github.com/spf13/cobra"
	"net/url"
)

func newNovelPartyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "party NAME",
		Short:       "Query authenticated party records by exact supplied name while preserving docket and API identifiers.",
		Example:     "  courtlistener-pp-cli party 'Example Corporation' --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			response, err := clGet(ctx, flags, "/parties/", url.Values{"name": {args[0]}, "page_size": {"100"}}, true)
			if err != nil {
				return err
			}
			return emitCL(cmd, flags, "live", map[string]any{"query": args[0], "parties": clResults(response), "next": response["next"], "match_note": "Preserve returned party and docket identifiers before treating records as the same litigant.", "caveats": clCaveats()})
		},
	}
	return cmd
}
