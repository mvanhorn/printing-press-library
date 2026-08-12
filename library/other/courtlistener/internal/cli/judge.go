// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import (
	"github.com/spf13/cobra"
	"net/url"
)

func newNovelJudgeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "judge PERSON_ID",
		Short:       "Return sourced judge/person metadata and clearly prohibit outcome prediction or causal scoring.",
		Example:     "  courtlistener-pp-cli judge 12345 --agent",
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
			person, err := clGet(ctx, flags, "/people/"+url.PathEscape(args[0])+"/", nil, true)
			if err != nil {
				return err
			}
			return emitCL(cmd, flags, "live", map[string]any{"person": person, "interpretation": "Sourced biographical and role metadata only; no outcome, ideology, or causal scoring.", "caveats": clCaveats()})
		},
	}
	return cmd
}
