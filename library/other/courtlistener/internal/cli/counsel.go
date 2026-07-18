// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import (
	"github.com/spf13/cobra"
	"net/url"
)

func newNovelCounselCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "counsel NAME",
		Short:       "Query authenticated attorney records by supplied name and retain observed docket and party relationships.",
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
			response, err := clGet(ctx, flags, "/attorneys/", url.Values{"name": {args[0]}, "page_size": {"100"}}, true)
			if err != nil {
				return err
			}
			return emitCL(cmd, flags, "live", map[string]any{"query": args[0], "attorneys": clResults(response), "next": response["next"], "match_note": "Name matches are observed records, not identity resolution; inspect API IDs and docket links.", "caveats": clCaveats()})
		},
	}
	return cmd
}
