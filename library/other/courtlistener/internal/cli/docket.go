// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call through clGet in courtlistener_novel_support.go

package cli

import "github.com/spf13/cobra"

func newNovelDocketCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "docket DOCKET_ID",
		Short:       "Join one authenticated docket with bounded entries, documents, parties, and counsel in source chronology.",
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
			bundle, err := docketBundle(ctx, flags, args[0])
			if err != nil {
				return err
			}
			bundle["coverage_caveats"] = clCaveats()
			return emitCL(cmd, flags, "live", bundle)
		},
	}
	return cmd
}
