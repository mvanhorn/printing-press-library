// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: sync coverage — resource_type counts + last_synced.

package cli

// pp:data-source local

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelSyncCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Report which market query keys are present, row counts, and last-synced times.",
		Example:     "  peerspace-pp-cli sync coverage --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := openNovelStoreRO(ctx, flagDB)
			if err != nil {
				return err
			}
			if s == nil {
				missingDBHint(flagDB)
				return printJSONFiltered(cmd.OutOrStdout(), make([]any, 0), flags)
			}
			defer s.Close()

			rows, err := loadCoverage(ctx, s)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "resource_type\tcount\tlast_synced")
				for _, r := range rows {
					ls, _ := r["last_synced"].(string)
					fmt.Fprintf(cmd.OutOrStdout(), "%v\t%v\t%s\n", r["resource_type"], r["count"], ls)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database path")
	return cmd
}
