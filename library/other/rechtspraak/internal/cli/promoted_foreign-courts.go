// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newForeignCourtsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "foreign-courts",
		Short:       "Foreign courts vocabulary (InstantiesBuitenlands, ~5000 entries)",
		Long:        "ECHR, CJEU, and EU member-state courts referenced by Dutch decisions. Cached locally.",
		Example:     "  rechtspraak-pp-cli foreign-courts\n  rechtspraak-pp-cli foreign-courts --json",
		Annotations: map[string]string{"pp:endpoint": "foreign-courts.list", "pp:method": "GET", "pp:path": "/Waardelijst/InstantiesBuitenlands", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			idx, err := getForeignCourtIndex(cmd.Context())
			if err != nil {
				return err
			}
			courts := idx.All()
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), courts)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d foreign courts (use --json for full structured output)\n", len(courts))
			limit := 20
			if len(courts) < limit {
				limit = len(courts)
			}
			for _, c := range courts[:limit] {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", c.Name)
			}
			if len(courts) > limit {
				fmt.Fprintf(cmd.OutOrStdout(), "  ... (%d more)\n", len(courts)-limit)
			}
			return nil
		},
	}
	return cmd
}
